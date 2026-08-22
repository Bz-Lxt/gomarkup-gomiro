package ws

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"gomiro/internal/cluster"
	"gomiro/internal/config"
	"gomiro/internal/engine"
	"gomiro/internal/logx"
	"gomiro/internal/obs"
	"gomiro/internal/model"
	"gomiro/internal/protocol"
	"gomiro/internal/store"
)

type inbound struct {
	client *Client
	env    protocol.Envelope
	raw    []byte
	bin    []byte
}

type Room struct {
	ID   string
	cfg  config.Config
	db   *store.DB
	bus  *cluster.Bus
	met  *obs.Metrics

	doc *engine.Document

	inbound chan inbound
	join    chan *Client
	leave   chan *Client

	clients map[string]*Client
	nextIdx uint32
	owner   bool

	stop     chan struct{}
	stopped  sync.Once
	unsub    func()
	lastSnap time.Time
	emptyAt  time.Time
	alive    atomic.Bool
}

func newRoom(id string, cfg config.Config, db *store.DB, bus *cluster.Bus, met *obs.Metrics) *Room {
	return &Room{
		ID:      id,
		cfg:     cfg,
		db:      db,
		bus:     bus,
		met:     met,
		doc:     engine.NewDocument(id),
		inbound: make(chan inbound, 1024),
		join:    make(chan *Client, 64),
		leave:   make(chan *Client, 64),
		clients: map[string]*Client{},
		stop:    make(chan struct{}),
	}
}

func (r *Room) start(ctx context.Context) {
	r.alive.Store(true)
	if snap, err := r.db.RebuildDocument(ctx, r.ID); err == nil {
		r.doc = snap
	} else {
		logx.L().Warn("rebuild document", "board", r.ID, "err", err)
	}
	if r.bus != nil {
		own, err := r.bus.TryOwn(ctx, r.ID)
		if err == nil {
			r.owner = own
		}
		unsub, err := r.bus.SubscribeBoard(ctx, r.ID, cluster.Handlers{
			OnInbox:  r.onInbox,
			OnOut:    r.onOut,
			OnCursor: r.onRemoteCursor,
		})
		if err == nil {
			r.unsub = unsub
		}
	} else {
		r.owner = true
	}
	go r.loop(ctx)
}

func (r *Room) Close() {
	r.stopped.Do(func() {
		close(r.stop)
		if r.unsub != nil {
			r.unsub()
		}
		if r.bus != nil && r.owner {
			r.bus.Release(context.Background(), r.ID)
		}
		r.persist(context.Background(), true)
		r.alive.Store(false)
	})
}

func (r *Room) loop(ctx context.Context) {
	cursorTick := time.NewTicker(r.cfg.CursorTick)
	snapTick := time.NewTicker(r.cfg.SnapshotInterval)
	ownTick := time.NewTicker(cluster.OwnerRenewPeriod())
	defer cursorTick.Stop()
	defer snapTick.Stop()
	defer ownTick.Stop()
	for {
		select {
		case <-r.stop:
			return
		case <-ctx.Done():
			return
		case c := <-r.join:
			r.handleJoin(c)
		case c := <-r.leave:
			r.handleLeave(c)
		case msg := <-r.inbound:
			r.handleInbound(msg)
		case <-cursorTick.C:
			r.flushCursors()
		case <-snapTick.C:
			r.persist(ctx, false)
			r.maybeReclaim()
		case <-ownTick.C:
			r.renewOwner(ctx)
		}
	}
}

func (r *Room) handleJoin(c *Client) {
	c.UserIdx = atomic.AddUint32(&r.nextIdx, 1)
	r.clients[c.ID] = c
	members := r.memberList()
	var missed []protocol.OpBroadcast
	if c.LastSeq > 0 && c.LastSeq < r.doc.ServerSeq && r.doc.ServerSeq-c.LastSeq < 2000 {
		if ops, err := r.db.OpsAfter(context.Background(), r.ID, c.LastSeq, 2000); err == nil {
			missed = ops
		}
	}
	snap := r.doc.Snapshot()
	payload := protocol.JoinedPayload{
		SelfID:    c.ID,
		UserIdx:   c.UserIdx,
		Members:   members,
		Snapshot:  snap,
		ServerSeq: r.doc.ServerSeq,
		Missed:    missed,
	}
	b, _ := protocol.Encode(protocol.TypeJoined, "", payload)
	c.EnqueueText(b)
	c.joined.Store(true)
	evt, _ := protocol.Encode(protocol.TypeMemberJoin, "", protocol.MemberEvent{Member: c.Member()})
	r.broadcastExcept(c.ID, evt)
	r.publishOut(protocol.TypeMemberJoin, evt)
	logx.L().Info("member join", "board", r.ID, "client", c.ID, "nick", c.Nickname)
}

func (r *Room) handleLeave(c *Client) {
	if _, ok := r.clients[c.ID]; !ok {
		return
	}
	delete(r.clients, c.ID)
	c.Close()
	evt, _ := protocol.Encode(protocol.TypeMemberLeave, "", protocol.MemberEvent{Member: c.Member()})
	r.broadcastExcept(c.ID, evt)
	r.publishOut(protocol.TypeMemberLeave, evt)
	if len(r.clients) == 0 {
		r.emptyAt = time.Now()
	}
	logx.L().Info("member leave", "board", r.ID, "client", c.ID)
}

func (r *Room) handleInbound(msg inbound) {
	c := msg.client
	if c == nil || c.Closed() {
		return
	}
	if len(msg.bin) > 0 {
		x, y, ok := protocol.DecodeCursorC2S(msg.bin)
		if !ok {
			c.EnqueueText(encodeErr(protocol.ErrBadField.String(), "bad cursor frame"))
			return
		}
		c.SetCursor(x, y)
		return
	}
	switch msg.env.Type {
	case protocol.TypePing:
		b, _ := protocol.Encode(protocol.TypePong, msg.env.ID, nil)
		c.EnqueueText(b)
	case protocol.TypeSelection:
		p, err := protocol.DecodePayload[protocol.SelectionPayload](msg.env.Payload)
		if err != nil || protocol.ValidateSelection(p.ShapeIDs) != nil {
			c.EnqueueText(encodeErr(protocol.ErrBadField.String(), "bad selection"))
			return
		}
		c.SetSel(p.ShapeIDs)
		raw, _ := protocol.Encode("selection_bcast", "", map[string]any{"clientId": c.ID, "shapeIds": p.ShapeIDs})
		r.broadcastExcept(c.ID, raw)
		r.publishOut("selection_bcast", raw)
	case protocol.TypeOp:
		if !c.noteOp(time.Now(), r.cfg.MaxOpPerSec) {
			c.EnqueueText(encodeErr(protocol.ErrRateLimited.String(), "too many ops"))
			return
		}
		r.handleOp(c, msg.env)
	default:
		c.EnqueueText(encodeErr(protocol.ErrBadType.String(), "unexpected type after join"))
	}
}

func (r *Room) handleOp(c *Client, env protocol.Envelope) {
	if r.bus != nil && !r.owner {
		raw, _ := json.Marshal(env)
		_ = r.bus.PublishInbox(context.Background(), cluster.InboxMsg{
			BoardID: r.ID, NodeID: r.cfg.NodeID, Client: c.ID, Kind: "op", Raw: raw,
		})
		return
	}
	p, err := protocol.DecodePayload[protocol.OpPayload](env.Payload)
	if err != nil {
		c.EnqueueText(encodeErr(protocol.ErrBadJSON.String(), "bad op"))
		return
	}
	if err := protocol.ValidateOp(p); err != nil {
		c.EnqueueText(encodeErr(protocol.ErrBadField.String(), err.Error()))
		return
	}
	if p.Lamport+1 > c.Lamport {
		c.Lamport = p.Lamport + 1
	}
	start := time.Now()
	res, err := r.applyOp(c.ID, p)
	if err != nil {
		c.EnqueueText(encodeErr(protocol.ErrBadField.String(), err.Error()))
		return
	}
	if r.met != nil {
		r.met.OpsTotal.Add(1)
		r.met.ObserveBroadcast(time.Since(start))
	}
	switch res.Decision {
	case engine.Idempotent:
		ack, _ := protocol.Encode(protocol.TypeOpAck, env.ID, protocol.OpAckPayload{
			ClientOpID: p.ClientOpID, ServerSeq: res.Seq, AcceptedVersion: res.Version,
		})
		c.EnqueueText(ack)
	case engine.Reject:
		if r.met != nil {
			r.met.OpsRejected.Add(1)
		}
		rej, _ := protocol.Encode(protocol.TypeOpReject, env.ID, protocol.OpRejectPayload{
			ClientOpID: p.ClientOpID, Reason: res.Reason, AuthoritativeShape: res.Shape,
		})
		c.EnqueueText(rej)
	case engine.Accept:
		ack, _ := protocol.Encode(protocol.TypeOpAck, env.ID, protocol.OpAckPayload{
			ClientOpID: p.ClientOpID, ServerSeq: res.Seq, AcceptedVersion: res.Version,
		})
		c.EnqueueText(ack)
		r.publishOut(protocol.TypeOpAck, ack)
		bcast := protocol.OpBroadcast{
			ServerSeq: res.Seq, AuthorID: c.ID, Kind: res.Kind,
			TargetID: res.TargetID, Patch: res.Patch, Lamport: p.Lamport, Version: res.Version,
		}
		raw, _ := protocol.Encode(protocol.TypeOpBcast, "", bcast)
		r.broadcastExcept(c.ID, raw)
		r.publishOut(protocol.TypeOpBcast, raw)
		payload, _ := json.Marshal(bcast)
		_ = r.db.AppendOp(context.Background(), r.ID, res.Seq, c.ID, res.Kind, payload)
	}
}

func (r *Room) applyOp(author string, p protocol.OpPayload) (engine.Result, error) {
	switch p.Kind {
	case protocol.OpCreate:
		cp, err := protocol.DecodePayload[protocol.CreatePatch](p.Patch)
		if err != nil {
			return engine.Result{}, err
		}
		if err := protocol.SanitizeCreate(cp.Shape); err != nil {
			return engine.Result{}, err
		}
		return r.doc.ApplyCreate(author, p.ClientOpID, cp.Shape)
	case protocol.OpUpdate:
		patch, err := protocol.SanitizeUpdatePatch(p.Patch)
		if err != nil {
			return engine.Result{}, err
		}
		tid := p.TargetID
		return r.doc.ApplyUpdate(author, p.ClientOpID, p.BaseVersion, tid, patch)
	case protocol.OpDelete:
		dp, err := protocol.DecodePayload[protocol.DeletePatch](p.Patch)
		if err != nil {
			// allow single targetId
			if p.TargetID != "" {
				dp.IDs = []string{p.TargetID}
			} else {
				return engine.Result{}, err
			}
		}
		return r.doc.ApplyDelete(author, p.ClientOpID, dp.IDs)
	case protocol.OpReorder:
		rp, err := protocol.DecodePayload[protocol.ReorderPatch](p.Patch)
		if err != nil {
			return engine.Result{}, err
		}
		id := rp.ID
		if id == "" {
			id = p.TargetID
		}
		var ez *float64
		if rp.Place == "" && rp.Z != 0 {
			z := rp.Z
			ez = &z
		}
		return r.doc.ApplyReorder(author, p.ClientOpID, id, rp.Place, ez)
	case protocol.OpGroup:
		gp, err := protocol.DecodePayload[protocol.GroupPatch](p.Patch)
		if err != nil {
			return engine.Result{}, err
		}
		return r.doc.ApplyGroup(author, p.ClientOpID, gp.GroupID, gp.IDs)
	case protocol.OpUngroup:
		gp, err := protocol.DecodePayload[protocol.GroupPatch](p.Patch)
		if err != nil {
			return engine.Result{}, err
		}
		return r.doc.ApplyUngroup(author, p.ClientOpID, gp.GroupID)
	default:
		return engine.Result{Decision: engine.Reject, Reason: protocol.RejectInvalid}, nil
	}
}

func (r *Room) flushCursors() {
	if len(r.clients) == 0 {
		return
	}
	samples := make([]protocol.CursorSample, 0, len(r.clients))
	for _, c := range r.clients {
		x, y := c.Cursor()
		samples = append(samples, protocol.CursorSample{UserIdx: c.UserIdx, X: x, Y: y})
	}
	frame := protocol.EncodeCursorS2C(samples)
	for _, c := range r.clients {
		if !c.EnqueueBin(frame, true) && r.met != nil {
			r.met.CursorsDrop.Add(1)
		}
	}
	if r.bus != nil {
		_ = r.bus.PublishCursor(context.Background(), cluster.CursorFan{
			BoardID: r.ID, NodeID: r.cfg.NodeID, Samples: samples,
		})
	}
}

func (r *Room) broadcastExcept(skip string, raw []byte) {
	for id, c := range r.clients {
		if id == skip {
			continue
		}
		if !c.EnqueueText(raw) && r.met != nil {
			r.met.SlowDisconnect.Add(1)
		}
	}
}

func (r *Room) publishOut(typ string, raw []byte) {
	if r.bus == nil || !r.owner {
		return
	}
	_ = r.bus.PublishOut(context.Background(), cluster.OutMsg{
		BoardID: r.ID, NodeID: r.cfg.NodeID, Type: typ, Envelope: raw,
	})
}

func (r *Room) onInbox(msg cluster.InboxMsg) {
	if !r.owner || msg.NodeID == r.cfg.NodeID {
		return
	}
	var env protocol.Envelope
	if json.Unmarshal(msg.Raw, &env) != nil {
		return
	}
	// Synthesize a proxy client so apply still records author.
	proxy := &Client{ID: msg.Client, BoardID: r.ID}
	select {
	case r.inbound <- inbound{client: proxy, env: env}:
	default:
	}
}

func (r *Room) onOut(msg cluster.OutMsg) {
	if msg.Type == protocol.TypeOpBcast && len(msg.Envelope) > 0 {
		env, err := protocol.Decode(msg.Envelope)
		if err == nil {
			if b, err := protocol.DecodePayload[protocol.OpBroadcast](env.Payload); err == nil {
				if err := r.doc.ApplyRemote(b); err != nil {
					logx.L().Warn("replica hole", "board", r.ID, "err", err)
					r.resyncFromStore()
				}
			}
		}
	}
	if len(msg.Envelope) > 0 {
		for _, c := range r.clients {
			c.EnqueueText(msg.Envelope)
		}
	}
}

func (r *Room) onRemoteCursor(fan cluster.CursorFan) {
	if len(fan.Samples) == 0 {
		return
	}
	frame := protocol.EncodeCursorS2C(fan.Samples)
	for _, c := range r.clients {
		c.EnqueueBin(frame, true)
	}
}

func (r *Room) resyncFromStore() {
	if doc, err := r.db.RebuildDocument(context.Background(), r.ID); err == nil {
		r.doc = doc
		snap := r.doc.Snapshot()
		raw, _ := protocol.Encode(protocol.TypeResync, "", protocol.ResyncPayload{
			Reason: "seq_hole", Snapshot: snap, ServerSeq: r.doc.ServerSeq,
		})
		for _, c := range r.clients {
			c.EnqueueText(raw)
		}
	}
}

func (r *Room) persist(ctx context.Context, force bool) {
	if r.doc == nil || (!r.doc.Dirty && !force) {
		return
	}
	if !r.owner && r.bus != nil {
		return
	}
	snap := r.doc.Snapshot()
	if err := r.db.SaveSnapshot(ctx, snap); err != nil {
		logx.L().Error("snapshot", "board", r.ID, "err", err)
		return
	}
	_ = r.db.CompactOps(ctx, r.ID, snap.ServerSeq)
	_ = r.db.TouchBoard(ctx, r.ID)
	if thumb := engine.ThumbnailPNG(snap.Shapes); thumb != "" {
		_ = r.db.UpdateBoard(ctx, r.ID, "", "__keep__", thumb)
	}
	r.doc.Dirty = false
	r.lastSnap = time.Now()
}

func (r *Room) maybeReclaim() {
	if len(r.clients) > 0 {
		r.emptyAt = time.Time{}
		return
	}
	if r.emptyAt.IsZero() {
		r.emptyAt = time.Now()
		return
	}
	if time.Since(r.emptyAt) > r.cfg.EmptyRoomGrace {
		r.Close()
	}
}

func (r *Room) renewOwner(ctx context.Context) {
	if r.bus == nil {
		r.owner = true
		return
	}
	own, err := r.bus.TryOwn(ctx, r.ID)
	if err != nil {
		return
	}
	if own {
		r.bus.RenewOwn(ctx, r.ID)
	}
	r.owner = own
}

func (r *Room) memberList() []model.Member {
	out := make([]model.Member, 0, len(r.clients))
	for _, c := range r.clients {
		out = append(out, c.Member())
	}
	return out
}

func (r *Room) LocalCount() int { return len(r.clients) }

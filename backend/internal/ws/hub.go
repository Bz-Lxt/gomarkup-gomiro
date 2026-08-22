package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/bcrypt"
	"gomiro/internal/cluster"
	"gomiro/internal/config"
	"gomiro/internal/logx"
	"gomiro/internal/obs"
	"gomiro/internal/model"
	"gomiro/internal/protocol"
	"gomiro/internal/store"
	"gomiro/internal/timeutil"
)

type Hub struct {
	cfg      config.Config
	db       *store.DB
	bus      *cluster.Bus
	met      *obs.Metrics
	upgrader websocket.Upgrader

	mu    sync.Mutex
	rooms map[string]*Room

	closeOnce sync.Once
	stop      chan struct{}
}

func NewHub(cfg config.Config, db *store.DB, bus *cluster.Bus, met *obs.Metrics) *Hub {
	return &Hub{
		cfg:      cfg,
		db:       db,
		bus:      bus,
		met:      met,
		upgrader: NewUpgrader(cfg),
		rooms:    map[string]*Room{},
		stop:     make(chan struct{}),
	}
}

func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		logx.L().Warn("ws upgrade", "err", err)
		return
	}
	conn.SetReadLimit(int64(h.cfg.MaxMsgBytes))
	_ = conn.SetReadDeadline(time.Now().Add(h.cfg.PongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(h.cfg.PongWait))
	})
	c := newClient(conn, h.cfg)
	h.met.Connections.Add(1)
	defer h.met.Connections.Add(-1)

	writeStop := make(chan struct{})
	go c.writePump(writeStop)

	// First message must be join (text).
	if err := h.readJoin(c); err != nil {
		c.EnqueueText(encodeErr(protocol.ErrBadField.String(), err.Error()))
		time.Sleep(50 * time.Millisecond)
		close(writeStop)
		c.Close()
		return
	}
	room := h.attach(r.Context(), c)
	if room == nil {
		c.EnqueueText(encodeErr("join_failed", "cannot join board"))
		close(writeStop)
		c.Close()
		return
	}
	defer func() {
		select {
		case room.leave <- c:
		case <-time.After(2 * time.Second):
			c.Close()
		}
		close(writeStop)
	}()

	for {
		mt, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if len(data) > h.cfg.MaxMsgBytes {
			c.EnqueueText(encodeErr(protocol.ErrTooLarge.String(), "message too large"))
			return
		}
		if mt == websocket.BinaryMessage {
			select {
			case room.inbound <- inbound{client: c, bin: data}:
			default:
				h.met.CursorsDrop.Add(1)
			}
			continue
		}
		env, err := protocol.Decode(data)
		if err != nil {
			c.EnqueueText(encodeErr(protocol.ErrBadJSON.String(), "malformed json"))
			continue
		}
		if err := protocol.ValidateEnvelope(env, len(data), h.cfg.MaxMsgBytes); err != nil {
			if ve, ok := err.(*protocol.ValidationError); ok {
				c.EnqueueText(encodeErr(string(ve.Code), ve.Message))
			} else {
				c.EnqueueText(encodeErr(protocol.ErrBadField.String(), err.Error()))
			}
			continue
		}
		select {
		case room.inbound <- inbound{client: c, env: env, raw: data}:
		default:
			c.EnqueueText(encodeErr(protocol.ErrRateLimited.String(), "room congested"))
		}
	}
}

func (h *Hub) readJoin(c *Client) error {
	_ = c.conn.SetReadDeadline(time.Now().Add(8 * time.Second))
	mt, data, err := c.conn.ReadMessage()
	if err != nil {
		return err
	}
	if mt != websocket.TextMessage {
		return protocol.Fail(protocol.ErrBadType, "join must be json")
	}
	env, err := protocol.Decode(data)
	if err != nil {
		return protocol.Fail(protocol.ErrBadJSON, "malformed json")
	}
	if err := protocol.ValidateEnvelope(env, len(data), h.cfg.MaxMsgBytes); err != nil {
		return err
	}
	if env.Type != protocol.TypeJoin {
		return protocol.Fail(protocol.ErrBadType, "first message must be join")
	}
	p, err := protocol.DecodePayload[protocol.JoinPayload](env.Payload)
	if err != nil {
		return protocol.Fail(protocol.ErrBadJSON, "bad join payload")
	}
	if err := protocol.ValidateJoin(p); err != nil {
		return err
	}
	board, err := h.db.GetBoard(context.Background(), p.BoardID)
	if err != nil {
		return protocol.Fail(protocol.ErrBadField, "board not found")
	}
	if board.PassHash != "" {
		if bcrypt.CompareHashAndPassword([]byte(board.PassHash), []byte(p.Passcode)) != nil {
			return protocol.Fail(protocol.ErrBadField, "passcode rejected")
		}
	}
	c.BoardID = p.BoardID
	c.Nickname = model.ClampNickname(p.Nickname)
	c.Color = p.Color
	if !model.ValidColor(c.Color) {
		c.Color = palette(c.ID)
	}
	c.LastSeq = p.LastSeq
	_ = c.conn.SetReadDeadline(time.Now().Add(h.cfg.PongWait))
	return nil
}

func (h *Hub) attach(ctx context.Context, c *Client) *Room {
	h.mu.Lock()
	room, ok := h.rooms[c.BoardID]
	if !ok || !room.alive.Load() {
		room = newRoom(c.BoardID, h.cfg, h.db, h.bus, h.met)
		h.rooms[c.BoardID] = room
		h.met.Rooms.Add(1)
		room.start(ctx)
	}
	h.mu.Unlock()
	select {
	case room.join <- c:
		return room
	case <-time.After(3 * time.Second):
		return nil
	}
}

func (h *Hub) Sweep() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for id, room := range h.rooms {
		if !room.alive.Load() {
			delete(h.rooms, id)
			h.met.Rooms.Add(-1)
		}
	}
}

func (h *Hub) Shutdown(ctx context.Context) {
	h.closeOnce.Do(func() {
		close(h.stop)
		raw, _ := protocol.Encode(protocol.TypeShutdown, "", map[string]string{"at": timeutil.Format(timeutil.Now())})
		h.mu.Lock()
		rooms := make([]*Room, 0, len(h.rooms))
		for _, room := range h.rooms {
			for _, c := range room.clients {
				c.EnqueueText(raw)
			}
			rooms = append(rooms, room)
		}
		h.mu.Unlock()
		for _, room := range rooms {
			room.Close()
		}
	})
}

func (h *Hub) RoomCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.rooms)
}

type RoomStat struct {
	ID      string `json:"id"`
	Members int    `json:"members"`
	Seq     uint64 `json:"seq"`
	Live    int    `json:"live"`
	Owner   bool   `json:"owner"`
}

func (h *Hub) Stats() []RoomStat {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]RoomStat, 0, len(h.rooms))
	for id, r := range h.rooms {
		if r == nil || !r.alive.Load() {
			continue
		}
		seq := uint64(0)
		live := 0
		if r.doc != nil {
			seq = r.doc.ServerSeq
			live = r.doc.LiveCount()
		}
		out = append(out, RoomStat{ID: id, Members: r.LocalCount(), Seq: seq, Live: live, Owner: r.owner})
	}
	return out
}

func (h *Hub) HasRoom(id string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	r, ok := h.rooms[id]
	return ok && r != nil && r.alive.Load()
}

func palette(id string) string {
	colors := []string{"#c45c26", "#2a6b6b", "#8b3a4a", "#3d5a80", "#b08900", "#4a7c59", "#6b4c7a", "#9a3412"}
	n := 0
	for i := 0; i < len(id); i++ {
		n += int(id[i])
	}
	return colors[n%len(colors)]
}

// EnsureBoard creates a board if missing (used by HTTP create).
func EnsureBoardJSON(b model.Board) []byte {
	raw, _ := json.Marshal(b)
	return raw
}

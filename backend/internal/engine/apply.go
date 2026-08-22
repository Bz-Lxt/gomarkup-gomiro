package engine

import (
	"encoding/json"
	"fmt"

	"gomiro/internal/model"
	"gomiro/internal/protocol"
	"gomiro/internal/timeutil"
)

func (d *Document) ApplyCreate(author, clientOpID string, shape *model.Shape) (Result, error) {
	res := d.ResolveCreate(clientOpID, shape)
	if res.Decision != Accept {
		return res, nil
	}
	seq := d.nextSeqLocked()
	s := shape.Clone()
	s.EnsurePropVer()
	if s.Z == 0 {
		s.Z = NextZ(d.Shapes)
	}
	s.LastWriter = author
	s.UpdatedAt = timeutil.UnixMilli()
	s.Version = 1
	for _, f := range model.PatchFields {
		s.PropVer[f] = seq
	}
	d.put(s)
	d.MarkSeen(clientOpID, seq)
	raw, err := json.Marshal(protocol.CreatePatch{Shape: s.Clone()})
	if err != nil {
		return Result{}, err
	}
	res.Seq = seq
	res.Version = s.Version
	res.Patch = raw
	res.Shape = s.Clone()
	return res, nil
}

func (d *Document) ApplyUpdate(author, clientOpID string, base uint64, targetID string, patch map[string]any) (Result, error) {
	res := d.ResolveUpdate(author, clientOpID, base, targetID, patch)
	if res.Decision != Accept {
		return res, nil
	}
	s := d.Shapes[targetID]
	seq := d.nextSeqLocked()
	if err := applyFields(s, patch); err != nil {
		return Result{}, err
	}
	d.Touch(author, seq, s, res.Fields)
	if geomFields(patch) {
		_ = d.RefreshAttachments(targetID, seq, author)
	}
	d.Dirty = true
	d.MarkSeen(clientOpID, seq)
	raw, err := json.Marshal(patch)
	if err != nil {
		return Result{}, err
	}
	res.Seq = seq
	res.Version = s.Version
	res.Patch = raw
	res.Shape = s.Clone()
	return res, nil
}

func (d *Document) ApplyDelete(author, clientOpID string, ids []string) (Result, error) {
	res := d.ResolveDelete(clientOpID, ids)
	if res.Decision != Accept {
		return res, nil
	}
	seq := d.nextSeqLocked()
	deleted := make([]string, 0, len(ids))
	for _, id := range ids {
		s := d.Shapes[id]
		if s == nil || s.Deleted {
			continue
		}
		s.Deleted = true
		s.LastWriter = author
		s.UpdatedAt = timeutil.UnixMilli()
		s.Version++
		d.Dirty = true
		deleted = append(deleted, id)
		if s.GroupID != "" {
			d.removeFromGroup(s.GroupID, id)
		}
	}
	d.MarkSeen(clientOpID, seq)
	raw, err := json.Marshal(protocol.DeletePatch{IDs: deleted})
	if err != nil {
		return Result{}, err
	}
	res.Seq = seq
	res.Patch = raw
	res.Kind = protocol.OpDelete
	return res, nil
}

func (d *Document) ApplyReorder(author, clientOpID, id, place string, explicitZ *float64) (Result, error) {
	if seq, ok := d.Seen(clientOpID); ok {
		return Result{Decision: Idempotent, Seq: seq, Kind: protocol.OpReorder, TargetID: id}, nil
	}
	s := d.Shapes[id]
	if s == nil {
		return Result{Decision: Reject, Reason: protocol.RejectUnknown, TargetID: id}, nil
	}
	if s.Deleted {
		return Result{Decision: Reject, Reason: protocol.RejectDeleted, Shape: s.Clone(), TargetID: id}, nil
	}
	var z float64
	if explicitZ != nil {
		z = *explicitZ
	} else {
		var ok bool
		z, ok = RecomputeZ(d.Shapes, id, place)
		if !ok {
			return Result{Decision: Reject, Reason: protocol.RejectUnknown, TargetID: id}, nil
		}
	}
	seq := d.nextSeqLocked()
	s.Z = z
	d.Touch(author, seq, s, []string{"z"})
	d.Dirty = true
	d.MarkSeen(clientOpID, seq)
	raw, err := json.Marshal(map[string]any{"z": z})
	if err != nil {
		return Result{}, err
	}
	return Result{
		Decision: Accept,
		Kind:     protocol.OpReorder,
		TargetID: id,
		Seq:      seq,
		Version:  s.Version,
		Patch:    raw,
		Shape:    s.Clone(),
	}, nil
}

func (d *Document) ApplyGroup(author, clientOpID, groupID string, ids []string) (Result, error) {
	if seq, ok := d.Seen(clientOpID); ok {
		return Result{Decision: Idempotent, Seq: seq, Kind: protocol.OpGroup}, nil
	}
	if groupID == "" {
		groupID = model.NewID("g")
	}
	live := make([]string, 0, len(ids))
	for _, id := range ids {
		s := d.Shapes[id]
		if s != nil && !s.Deleted {
			live = append(live, id)
		}
	}
	if len(live) < 2 {
		return Result{Decision: Reject, Reason: protocol.RejectInvalid}, nil
	}
	seq := d.nextSeqLocked()
	d.Groups[groupID] = append([]string(nil), live...)
	for _, id := range live {
		s := d.Shapes[id]
		s.GroupID = groupID
		d.Touch(author, seq, s, []string{"groupId"})
	}
	d.Dirty = true
	d.MarkSeen(clientOpID, seq)
	raw, err := json.Marshal(protocol.GroupPatch{GroupID: groupID, IDs: live})
	if err != nil {
		return Result{}, err
	}
	return Result{Decision: Accept, Kind: protocol.OpGroup, TargetID: groupID, Seq: seq, Patch: raw}, nil
}

func (d *Document) ApplyUngroup(author, clientOpID, groupID string) (Result, error) {
	if seq, ok := d.Seen(clientOpID); ok {
		return Result{Decision: Idempotent, Seq: seq, Kind: protocol.OpUngroup}, nil
	}
	ids := d.Groups[groupID]
	seq := d.nextSeqLocked()
	for _, id := range ids {
		if s := d.Shapes[id]; s != nil {
			s.GroupID = ""
			d.Touch(author, seq, s, []string{"groupId"})
		}
	}
	delete(d.Groups, groupID)
	d.Dirty = true
	d.MarkSeen(clientOpID, seq)
	raw, err := json.Marshal(protocol.GroupPatch{GroupID: groupID, IDs: ids})
	if err != nil {
		return Result{}, err
	}
	return Result{Decision: Accept, Kind: protocol.OpUngroup, TargetID: groupID, Seq: seq, Patch: raw}, nil
}

func (d *Document) removeFromGroup(gid, id string) {
	ids := d.Groups[gid]
	out := ids[:0]
	for _, x := range ids {
		if x != id {
			out = append(out, x)
		}
	}
	if len(out) < 2 {
		for _, x := range out {
			if s := d.Shapes[x]; s != nil {
				s.GroupID = ""
			}
		}
		delete(d.Groups, gid)
		return
	}
	d.Groups[gid] = append([]string(nil), out...)
}

func applyFields(s *model.Shape, patch map[string]any) error {
	for k, v := range patch {
		switch k {
		case "x":
			s.X = v.(float64)
		case "y":
			s.Y = v.(float64)
		case "w":
			s.W = v.(float64)
		case "h":
			s.H = v.(float64)
		case "rotation":
			s.Rotation = v.(float64)
		case "stroke":
			s.Stroke = v.(string)
		case "fill":
			s.Fill = v.(string)
		case "strokeW":
			s.StrokeW = v.(float64)
		case "dash":
			s.Dash = v.(string)
		case "opacity":
			s.Opacity = v.(float64)
		case "z":
			s.Z = v.(float64)
		case "text":
			s.Text = v.(string)
		case "fontSize":
			s.FontSize = v.(float64)
		case "align":
			s.Align = v.(string)
		case "points":
			s.Points = v.([]model.Point)
		case "startId":
			s.StartID = v.(string)
		case "endId":
			s.EndID = v.(string)
		case "startAnchor":
			s.StartAnchor = v.(string)
		case "endAnchor":
			s.EndAnchor = v.(string)
		case "imageUrl":
			s.ImageURL = v.(string)
		case "groupId":
			s.GroupID = v.(string)
		case "radius":
			s.Radius = v.(float64)
		default:
			return fmt.Errorf("unknown field %s", k)
		}
	}
	return nil
}

// ApplyRemote applies an already-sequenced broadcast from the room owner.
// Used by replica nodes so they do not mint a new serverSeq.
func (d *Document) ApplyRemote(bcast protocol.OpBroadcast) error {
	if bcast.ServerSeq <= d.ServerSeq {
		return nil
	}
	if bcast.ServerSeq != d.ServerSeq+1 {
		return fmt.Errorf("seq hole: have %d got %d", d.ServerSeq, bcast.ServerSeq)
	}
	switch bcast.Kind {
	case protocol.OpCreate:
		p, err := protocol.DecodePayload[protocol.CreatePatch](bcast.Patch)
		if err != nil || p.Shape == nil {
			return fmt.Errorf("create patch: %w", err)
		}
		d.Shapes[p.Shape.ID] = p.Shape.Clone()
	case protocol.OpUpdate, protocol.OpReorder:
		patch, err := protocol.SanitizeUpdatePatch(bcast.Patch)
		if err != nil {
			// reorder patch is {z}
			var raw map[string]any
			if json.Unmarshal(bcast.Patch, &raw) == nil {
				if z, ok := raw["z"]; ok {
					if f, ok := asF(z); ok {
						if s := d.Shapes[bcast.TargetID]; s != nil {
							s.Z = f
							s.Version = bcast.Version
							s.LastWriter = bcast.AuthorID
						}
					}
				}
			} else {
				return err
			}
		} else if s := d.Shapes[bcast.TargetID]; s != nil {
			_ = applyFields(s, patch)
			s.Version = bcast.Version
			s.LastWriter = bcast.AuthorID
			s.EnsurePropVer()
			for k := range patch {
				s.PropVer[k] = bcast.ServerSeq
			}
		}
	case protocol.OpDelete:
		p, err := protocol.DecodePayload[protocol.DeletePatch](bcast.Patch)
		if err != nil {
			return err
		}
		for _, id := range p.IDs {
			if s := d.Shapes[id]; s != nil {
				s.Deleted = true
			}
		}
	case protocol.OpGroup:
		p, err := protocol.DecodePayload[protocol.GroupPatch](bcast.Patch)
		if err != nil {
			return err
		}
		d.Groups[p.GroupID] = append([]string(nil), p.IDs...)
		for _, id := range p.IDs {
			if s := d.Shapes[id]; s != nil {
				s.GroupID = p.GroupID
			}
		}
	case protocol.OpUngroup:
		p, err := protocol.DecodePayload[protocol.GroupPatch](bcast.Patch)
		if err != nil {
			return err
		}
		for _, id := range p.IDs {
			if s := d.Shapes[id]; s != nil {
				s.GroupID = ""
			}
		}
		delete(d.Groups, p.GroupID)
	}
	d.ServerSeq = bcast.ServerSeq
	d.Dirty = true
	return nil
}

func asF(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	default:
		return 0, false
	}
}

// Inverse builds a compensating op payload so a client can undo its own write
// without touching foreign shapes. The server never applies inverses itself.
func Inverse(kind string, before *model.Shape, patch map[string]any, ids []string, groupID string) (string, json.RawMessage) {
	switch kind {
	case protocol.OpCreate:
		raw, _ := json.Marshal(protocol.DeletePatch{IDs: []string{before.ID}})
		return protocol.OpDelete, raw
	case protocol.OpDelete:
		if before == nil {
			return "", nil
		}
		c := before.Clone()
		c.Deleted = false
		raw, _ := json.Marshal(protocol.CreatePatch{Shape: c})
		return protocol.OpCreate, raw
	case protocol.OpUpdate, protocol.OpReorder:
		if before == nil {
			return "", nil
		}
		inv := map[string]any{}
		for k := range patch {
			if n, ok := before.GetNumber(k); ok {
				inv[k] = n
				continue
			}
			if s, ok := before.GetString(k); ok {
				inv[k] = s
			}
		}
		raw, _ := json.Marshal(inv)
		return protocol.OpUpdate, raw
	case protocol.OpGroup:
		raw, _ := json.Marshal(protocol.GroupPatch{GroupID: groupID, IDs: ids})
		return protocol.OpUngroup, raw
	case protocol.OpUngroup:
		raw, _ := json.Marshal(protocol.GroupPatch{GroupID: groupID, IDs: ids})
		return protocol.OpGroup, raw
	default:
		return "", nil
	}
}

func DocumentsEqual(a, b *Document) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.ServerSeq != b.ServerSeq {
		return false
	}
	if len(a.Shapes) != len(b.Shapes) {
		// deleted shapes may linger; compare live only
	}
	ids := map[string]struct{}{}
	for id, s := range a.Shapes {
		if s != nil && !s.Deleted {
			ids[id] = struct{}{}
		}
	}
	for id, s := range b.Shapes {
		if s != nil && !s.Deleted {
			ids[id] = struct{}{}
		}
	}
	for id := range ids {
		as, bs := a.Shapes[id], b.Shapes[id]
		if as == nil || bs == nil || as.Deleted != bs.Deleted {
			return false
		}
		if as.Deleted {
			continue
		}
		if as.X != bs.X || as.Y != bs.Y || as.W != bs.W || as.H != bs.H {
			return false
		}
		if as.Fill != bs.Fill || as.Stroke != bs.Stroke || as.Text != bs.Text || as.Z != bs.Z {
			return false
		}
		if as.Rotation != bs.Rotation || as.Opacity != bs.Opacity || as.GroupID != bs.GroupID {
			return false
		}
		if as.Kind != bs.Kind || as.Dash != bs.Dash || as.StrokeW != bs.StrokeW {
			return false
		}
		if as.StartID != bs.StartID || as.EndID != bs.EndID {
			return false
		}
		if len(as.Points) != len(bs.Points) {
			return false
		}
		for i := range as.Points {
			if as.Points[i].X != bs.Points[i].X || as.Points[i].Y != bs.Points[i].Y {
				return false
			}
		}
	}
	if len(a.Groups) != len(b.Groups) {
		return false
	}
	for gid, idsA := range a.Groups {
		idsB := b.Groups[gid]
		if len(idsA) != len(idsB) {
			return false
		}
	}
	return true
}

type RebuildReport struct {
	BoardID   string
	FromSeq   uint64
	ToSeq     uint64
	Applied   int
	Skipped   int
	Live      int
	Hole      bool
	HoleAt    uint64
}

func (d *Document) ReplayBroadcasts(ops []protocol.OpBroadcast) RebuildReport {
	rep := RebuildReport{BoardID: d.BoardID, FromSeq: d.ServerSeq}
	for _, op := range ops {
		if op.ServerSeq <= d.ServerSeq {
			rep.Skipped++
			continue
		}
		if err := d.ApplyRemote(op); err != nil {
			rep.Hole = true
			rep.HoleAt = op.ServerSeq
			break
		}
		rep.Applied++
	}
	rep.ToSeq = d.ServerSeq
	rep.Live = d.LiveCount()
	return rep
}

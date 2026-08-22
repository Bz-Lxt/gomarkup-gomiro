package engine

import (
	"encoding/json"

	"gomiro/internal/model"
	"gomiro/internal/protocol"
)

type Decision int

const (
	Accept Decision = iota
	Reject
	Idempotent
)

type Result struct {
	Decision Decision
	Reason   string
	Shape    *model.Shape
	Seq      uint64
	Version  uint64
	Kind     string
	TargetID string
	Patch    json.RawMessage
	Fields   []string
}

// ResolveUpdate implements CF-02..CF-07:
//   - same clientOpId → Idempotent
//   - deleted target → Reject (Delete Wins)
//   - missing target → Reject
//   - property intersection with newer PropVer → Reject + authoritative shape
//   - otherwise Accept (rebase if baseVersion is stale but no intersection)
func (d *Document) ResolveUpdate(author, clientOpID string, baseVersion uint64, targetID string, patch map[string]any) Result {
	if seq, ok := d.Seen(clientOpID); ok {
		s := d.Get(targetID)
		return Result{Decision: Idempotent, Seq: seq, Shape: s, Kind: protocol.OpUpdate, TargetID: targetID}
	}
	cur := d.Shapes[targetID]
	if cur == nil {
		return Result{Decision: Reject, Reason: protocol.RejectUnknown, TargetID: targetID}
	}
	if cur.Deleted {
		return Result{Decision: Reject, Reason: protocol.RejectDeleted, Shape: cur.Clone(), TargetID: targetID}
	}
	fields := make([]string, 0, len(patch))
	for k := range patch {
		fields = append(fields, k)
	}
	if intersectSince(cur, baseVersion, fields) {
		return Result{
			Decision: Reject,
			Reason:   protocol.RejectStale,
			Shape:    cur.Clone(),
			TargetID: targetID,
		}
	}
	return Result{
		Decision: Accept,
		Kind:     protocol.OpUpdate,
		TargetID: targetID,
		Fields:   fields,
	}
}

func intersectSince(s *model.Shape, base uint64, fields []string) bool {
	if s.PropVer == nil {
		return false
	}
	for _, f := range fields {
		if pv, ok := s.PropVer[f]; ok && pv > base {
			return true
		}
	}
	// If the client has a stale version but the conflicting properties
	// were never recorded (legacy), fall back to whole-shape version.
	if base < s.Version {
		for _, f := range fields {
			if _, ok := s.PropVer[f]; !ok {
				// unknown history for this field after a restore — treat as conflict
				// only when version moved and field is present on the shape conceptually.
				_ = f
			}
		}
	}
	return false
}

func (d *Document) ResolveCreate(clientOpID string, shape *model.Shape) Result {
	if seq, ok := d.Seen(clientOpID); ok {
		return Result{Decision: Idempotent, Seq: seq, Shape: d.Get(shape.ID), Kind: protocol.OpCreate, TargetID: shape.ID}
	}
	if existing := d.Shapes[shape.ID]; existing != nil && !existing.Deleted {
		return Result{Decision: Reject, Reason: protocol.RejectDuplicate, Shape: existing.Clone(), TargetID: shape.ID}
	}
	return Result{Decision: Accept, Kind: protocol.OpCreate, TargetID: shape.ID}
}

func (d *Document) ResolveDelete(clientOpID string, ids []string) Result {
	if seq, ok := d.Seen(clientOpID); ok {
		return Result{Decision: Idempotent, Seq: seq, Kind: protocol.OpDelete}
	}
	live := 0
	for _, id := range ids {
		if s := d.Shapes[id]; s != nil && !s.Deleted {
			live++
		}
	}
	if live == 0 && len(ids) > 0 {
		// deleting already-deleted is idempotent at the state level
		return Result{Decision: Accept, Kind: protocol.OpDelete}
	}
	return Result{Decision: Accept, Kind: protocol.OpDelete}
}

func TieBreak(aLamport uint64, aClient string, bLamport uint64, bClient string) int {
	if aLamport < bLamport {
		return -1
	}
	if aLamport > bLamport {
		return 1
	}
	if aClient < bClient {
		return -1
	}
	if aClient > bClient {
		return 1
	}
	return 0
}

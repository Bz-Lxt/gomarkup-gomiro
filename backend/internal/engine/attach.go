package engine

import (
	"gomiro/internal/model"
)

// RefreshAttachments walks every arrow/line that pins to moved and rewrites
// its world-space endpoints. Called after a geometry mutation so remote clients
// do not have to recompute anchors themselves.
func (d *Document) RefreshAttachments(movedID string, seq uint64, author string) []string {
	if movedID == "" {
		return nil
	}
	moved := d.Shapes[movedID]
	if moved == nil || moved.Deleted {
		return nil
	}
	touched := make([]string, 0, 4)
	for id, s := range d.Shapes {
		if s == nil || s.Deleted {
			continue
		}
		if s.Kind != model.KindArrow && s.Kind != model.KindLine {
			continue
		}
		changed := false
		if s.StartID == movedID {
			x, y := AnchorPoint(moved, s.StartAnchor)
			if len(s.Points) >= 2 {
				s.Points[0] = model.Point{X: x - s.X, Y: y - s.Y}
			} else {
				s.X, s.Y = x, y
			}
			changed = true
		}
		if s.EndID == movedID {
			x, y := AnchorPoint(moved, s.EndAnchor)
			if len(s.Points) >= 2 {
				s.Points[len(s.Points)-1] = model.Point{X: x - s.X, Y: y - s.Y}
			} else {
				s.W = x - s.X
				s.H = y - s.Y
			}
			changed = true
		}
		if changed {
			d.Touch(author, seq, s, []string{"points", "x", "y", "w", "h"})
			touched = append(touched, id)
		}
	}
	return touched
}

func geomFields(patch map[string]any) bool {
	for _, k := range []string{"x", "y", "w", "h", "rotation"} {
		if _, ok := patch[k]; ok {
			return true
		}
	}
	return false
}

package engine

import (
	"encoding/json"
	"testing"

	"gomiro/internal/model"
	"gomiro/internal/protocol"
)

func TestPropertyLWWNoIntersection(t *testing.T) {
	d := NewDocument("b1")
	s := model.NewShape(model.KindRect, "shp_aaaaaaaaaa")
	s.X, s.Y = 10, 20
	if _, err := d.ApplyCreate("c1", "op_create01", s); err != nil {
		t.Fatal(err)
	}
	// A moves x
	r1, err := d.ApplyUpdate("c1", "op_move_x01", 1, s.ID, map[string]any{"x": 40.0})
	if err != nil || r1.Decision != Accept {
		t.Fatalf("move x: %+v %v", r1, err)
	}
	// B changes fill against baseVersion=1 (before x write). fill has no intersection.
	r2, err := d.ApplyUpdate("c2", "op_fill_001", 1, s.ID, map[string]any{"fill": "#ff0000"})
	if err != nil || r2.Decision != Accept {
		t.Fatalf("fill should rebase, got %+v %v", r2, err)
	}
	got := d.Get(s.ID)
	if got.X != 40 || got.Fill != "#ff0000" {
		t.Fatalf("expected both x and fill kept, got x=%v fill=%s", got.X, got.Fill)
	}
}

func TestPropertyIntersectionReject(t *testing.T) {
	d := NewDocument("b1")
	s := model.NewShape(model.KindRect, "shp_bbbbbbbbbb")
	if _, err := d.ApplyCreate("c1", "op_create02", s); err != nil {
		t.Fatal(err)
	}
	if _, err := d.ApplyUpdate("c1", "op_move_x02", 1, s.ID, map[string]any{"x": 80.0}); err != nil {
		t.Fatal(err)
	}
	r, err := d.ApplyUpdate("c2", "op_move_x03", 1, s.ID, map[string]any{"x": 12.0})
	if err != nil {
		t.Fatal(err)
	}
	if r.Decision != Reject || r.Reason != protocol.RejectStale {
		t.Fatalf("want stale reject, got %+v", r)
	}
	if d.Get(s.ID).X != 80 {
		t.Fatalf("authoritative x lost: %v", d.Get(s.ID).X)
	}
}

func TestDeleteWins(t *testing.T) {
	d := NewDocument("b1")
	s := model.NewShape(model.KindRect, "shp_cccccccccc")
	if _, err := d.ApplyCreate("c1", "op_create03", s); err != nil {
		t.Fatal(err)
	}
	if _, err := d.ApplyDelete("c1", "op_del_00001", []string{s.ID}); err != nil {
		t.Fatal(err)
	}
	r, err := d.ApplyUpdate("c2", "op_upd_dead1", 1, s.ID, map[string]any{"x": 9.0})
	if err != nil {
		t.Fatal(err)
	}
	if r.Decision != Reject || r.Reason != protocol.RejectDeleted {
		t.Fatalf("want delete-wins, got %+v", r)
	}
	if !d.Shapes[s.ID].Deleted {
		t.Fatal("shape resurrected")
	}
}

func TestIdempotentClientOp(t *testing.T) {
	d := NewDocument("b1")
	s := model.NewShape(model.KindEllipse, "shp_dddddddddd")
	r1, err := d.ApplyCreate("c1", "op_same_0001", s)
	if err != nil {
		t.Fatal(err)
	}
	r2, err := d.ApplyCreate("c1", "op_same_0001", s)
	if err != nil {
		t.Fatal(err)
	}
	if r2.Decision != Idempotent || r2.Seq != r1.Seq {
		t.Fatalf("want idempotent same seq, got %+v vs %+v", r2, r1)
	}
	if d.LiveCount() != 1 {
		t.Fatalf("live=%d", d.LiveCount())
	}
}

func TestConvergenceRandom(t *testing.T) {
	d := NewDocument("b1")
	s := model.NewShape(model.KindRect, "shp_eeeeeeeeee")
	if _, err := d.ApplyCreate("seed", "op_seed_0001", s); err != nil {
		t.Fatal(err)
	}
	clients := []string{"cA", "cB", "cC"}
	for i := 0; i < 80; i++ {
		author := clients[i%len(clients)]
		base := d.Get(s.ID).Version
		patch := map[string]any{}
		switch i % 4 {
		case 0:
			patch["x"] = float64(i)
		case 1:
			patch["y"] = float64(i * 2)
		case 2:
			patch["fill"] = []string{"#aa0000", "#00aa00", "#0000aa"}[i%3]
		default:
			patch["rotation"] = float64(i % 360)
		}
		_, _ = d.ApplyUpdate(author, model.NewID("op"), base, s.ID, patch)
	}
	// Replay the accepted op log onto a fresh replica via ApplyRemote using snapshots
	// of current state — all sequential applies already linearized; clone must match.
	snap := d.Snapshot()
	clone := NewDocument("b1")
	clone.Restore(snap)
	a, _ := json.Marshal(d.Snapshot().Shapes[s.ID])
	b, _ := json.Marshal(clone.Snapshot().Shapes[s.ID])
	if string(a) != string(b) {
		t.Fatalf("replica diverge\n%s\n%s", a, b)
	}
}

func TestTieBreak(t *testing.T) {
	if TieBreak(1, "a", 2, "a") >= 0 {
		t.Fatal("lamport should win")
	}
	if TieBreak(3, "b", 3, "a") <= 0 {
		t.Fatal("client id should break ties")
	}
}

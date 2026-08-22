package engine

import (
	"testing"
	"time"

	"gomiro/internal/model"
)

// Regression: switching data sources clears the in-memory document with an
// empty (nil) snapshot via Restore(nil). The original implementation only
// registered `defer d.mu.Unlock()` *after* the nil-snapshot early return,
// so Restore(nil) returned while still holding the write lock. Every later
// read (Get/Snapshot/Apply*) blocked forever — process alive, no panic,
// only the affected board stuck (each room owns its own Document/mutex).

func TestRestoreNilSnapshotDoesNotDeadlock(t *testing.T) {
	d := NewDocument("b1")
	// Populate so a stale lock would be observable.
	s := model.NewShape(model.KindRect, "shp_lock_0001")
	if _, err := d.ApplyCreate("c1", "op_lock_0001", s); err != nil {
		t.Fatal(err)
	}

	// Switch data source: clear in-memory state with an empty snapshot.
	d.Restore(nil)

	if d.LiveCount() != 0 {
		t.Fatalf("expected empty doc after Restore(nil), live=%d", d.LiveCount())
	}
	if d.ServerSeq != 0 {
		t.Fatalf("expected seq 0 after Restore(nil), got %d", d.ServerSeq)
	}

	// Reads must not block. Use a timeout to turn a deadlock into a failure.
	reads := map[string]func(){
		"Get":      func() { d.Get("shp_lock_0001") },
		"Snapshot": func() { d.Snapshot() },
		"LiveCount": func() { d.LiveCount() },
		"CloneSeen": func() { d.CloneSeen() },
	}
	for name, fn := range reads {
		done := make(chan struct{})
		go func() { fn(); close(done) }()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("%s blocked after Restore(nil): mutex not released", name)
		}
	}
}

// After clearing, the document must be usable again: a new create should
// apply and become readable, and a snapshot should reflect it.

func TestRestoreNilSnapshotThenRecover(t *testing.T) {
	d := NewDocument("b1")
	old := model.NewShape(model.KindRect, "shp_old_0001")
	if _, err := d.ApplyCreate("c1", "op_old_0001", old); err != nil {
		t.Fatal(err)
	}

	d.Restore(nil)

	fresh := model.NewShape(model.KindEllipse, "shp_new_0001")
	res, err := d.ApplyCreate("c2", "op_new_0001", fresh)
	if err != nil {
		t.Fatalf("create after clear: %v", err)
	}
	if res.Decision != Accept {
		t.Fatalf("expected accept, got %+v", res)
	}
	if got := d.Get(fresh.ID); got == nil {
		t.Fatal("fresh shape not readable after clear+create")
	}
	snap := d.Snapshot()
	if len(snap.Shapes) != 1 {
		t.Fatalf("expected 1 shape in snapshot, got %d", len(snap.Shapes))
	}
	if snap.ServerSeq != 1 {
		t.Fatalf("expected seq 1, got %d", snap.ServerSeq)
	}
}

// A non-nil empty snapshot (zero shapes, zero seq) must behave the same as
// nil for recovery purposes — this guards the common "switch source" path
// that sends an explicit empty snapshot.

func TestRestoreEmptySnapshotThenRecover(t *testing.T) {
	d := NewDocument("b1")
	old := model.NewShape(model.KindRect, "shp_old_0002")
	if _, err := d.ApplyCreate("c1", "op_old_0002", old); err != nil {
		t.Fatal(err)
	}

	empty := &model.Snapshot{BoardID: d.BoardID, Shapes: map[string]*model.Shape{}}
	d.Restore(empty)

	if d.LiveCount() != 0 || d.ServerSeq != 0 {
		t.Fatalf("expected cleared doc, live=%d seq=%d", d.LiveCount(), d.ServerSeq)
	}
	fresh := model.NewShape(model.KindSticky, "shp_new_0002")
	if _, err := d.ApplyCreate("c3", "op_new_0002", fresh); err != nil {
		t.Fatalf("create after empty restore: %v", err)
	}
	if got := d.Snapshot(); len(got.Shapes) != 1 {
		t.Fatalf("expected 1 shape, got %d", len(got.Shapes))
	}
}

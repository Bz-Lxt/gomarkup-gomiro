package engine_test

import (
	"testing"
	"time"

	"gomiro/internal/engine"
)

func TestRestoreNilLeavesDocumentUsable(t *testing.T) {
	doc := engine.NewDocument("board-1")
	doc.Restore(nil)

	done := make(chan struct{})
	go func() {
		_ = doc.Get("missing-shape")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("document remained locked after restoring an empty snapshot")
	}
}

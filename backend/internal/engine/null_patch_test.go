package engine_test

import (
	"encoding/json"
	"testing"

	"gomiro/internal/engine"
	"gomiro/internal/model"
	"gomiro/internal/protocol"
)

func TestNullUpdatePatchDoesNotAdvanceDocument(t *testing.T) {
	doc := engine.NewDocument("b_test")
	shape := model.NewShape(model.KindRect, "s_test")
	created, err := doc.ApplyCreate("client_a", "create_1", shape)
	if err != nil || created.Decision != engine.Accept {
		t.Fatalf("create shape: result=%+v err=%v", created, err)
	}

	before := doc.Get(shape.ID)
	beforeSeq := doc.ServerSeq
	patch, err := protocol.SanitizeUpdatePatch(json.RawMessage("null"))
	if err == nil {
		updated, applyErr := doc.ApplyUpdate("client_a", "update_1", before.Version, shape.ID, patch)
		t.Fatalf("null patch accepted: result=%+v applyErr=%v seq=%d (before %d)", updated, applyErr, doc.ServerSeq, beforeSeq)
	}

	if doc.ServerSeq != beforeSeq {
		t.Fatalf("rejected update advanced server sequence: got %d want %d", doc.ServerSeq, beforeSeq)
	}
	after := doc.Get(shape.ID)
	if after.Version != before.Version {
		t.Fatalf("rejected update changed shape version: got %d want %d", after.Version, before.Version)
	}
}

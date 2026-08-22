package engine_test

import (
	"testing"

	"gomiro/internal/engine"
	"gomiro/internal/model"
)

func TestRestoreDetachesGroupMembersFromSnapshot(t *testing.T) {
	members := []string{"shape-a", "shape-b"}
	snapshot := &model.Snapshot{
		BoardID: "board-1",
		Groups:  map[string][]string{"group-1": members},
	}

	doc := engine.NewDocument("board-1")
	doc.Restore(snapshot)

	members[0] = "shape-from-reused-buffer"
	members = append(members[:0], "shape-from-compacted-cache")

	restored := doc.Snapshot()
	got := restored.Groups["group-1"]
	if len(got) != 2 || got[0] != "shape-a" || got[1] != "shape-b" {
		t.Fatalf("restored group changed after its input snapshot was reused: got %v, want [shape-a shape-b]", got)
	}
}

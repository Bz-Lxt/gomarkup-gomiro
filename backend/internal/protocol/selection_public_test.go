package protocol_test

import (
	"testing"

	"gomiro/internal/protocol"
)

func TestValidateInboundAcceptsSelection(t *testing.T) {
	tests := []struct {
		name string
		ids  []string
	}{
		{name: "one shape", ids: []string{"shp_abcdef"}},
		{name: "clear selection", ids: []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := protocol.Encode(protocol.TypeSelection, "msg_selection", protocol.SelectionPayload{ShapeIDs: tt.ids})
			if err != nil {
				t.Fatalf("encode valid selection: %v", err)
			}
			if err := protocol.ValidateInbound(raw, false, 4096); err != nil {
				t.Fatal("valid selection was rejected")
			}
		})
	}
}

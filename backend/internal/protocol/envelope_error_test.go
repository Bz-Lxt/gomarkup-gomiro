package protocol_test

import (
	"encoding/json"
	"errors"
	"testing"

	"gomiro/internal/protocol"
)

var errPayloadUnavailable = errors.New("payload temporarily unavailable")

type unavailablePayload struct{}

func (unavailablePayload) MarshalJSON() ([]byte, error) {
	return nil, errPayloadUnavailable
}

func TestEncodePreservesPayloadError(t *testing.T) {
	_, err := protocol.Encode(protocol.TypeOpBcast, "msg-1", unavailablePayload{})
	if err == nil {
		t.Fatal("Encode returned nil error for an unencodable payload")
	}
	if !errors.Is(err, errPayloadUnavailable) {
		t.Fatalf("Encode error lost the payload failure: %v", err)
	}

	var marshalerErr *json.MarshalerError
	if !errors.As(err, &marshalerErr) {
		t.Fatalf("Encode error lost json.MarshalerError context: %v", err)
	}
}

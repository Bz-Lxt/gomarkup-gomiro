package protocol

import (
	"encoding/json"

	"gomiro/internal/model"
)

type Envelope struct {
	V       int             `json:"v"`
	Type    string          `json:"type"`
	ID      string          `json:"id,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type JoinPayload struct {
	BoardID      string `json:"boardId"`
	Nickname     string `json:"nickname"`
	Color        string `json:"color"`
	Passcode     string `json:"passcode,omitempty"`
	LastSeq      uint64 `json:"lastSeq"`
	ProtoVersion int    `json:"protoVersion"`
}

type OpPayload struct {
	ClientOpID  string          `json:"clientOpId"`
	Lamport     uint64          `json:"lamport"`
	BaseVersion uint64          `json:"baseVersion"`
	Kind        string          `json:"opKind"`
	TargetID    string          `json:"targetId"`
	Patch       json.RawMessage `json:"patch,omitempty"`
}

type SelectionPayload struct {
	ShapeIDs []string `json:"shapeIds"`
}

type JoinedPayload struct {
	SelfID    string          `json:"selfId"`
	UserIdx   uint32          `json:"userIdx"`
	Members   []model.Member  `json:"members"`
	Snapshot  *model.Snapshot `json:"snapshot"`
	ServerSeq uint64          `json:"serverSeq"`
	Missed    []OpBroadcast   `json:"missed,omitempty"`
}

type OpAckPayload struct {
	ClientOpID        string `json:"clientOpId"`
	ServerSeq         uint64 `json:"serverSeq"`
	AcceptedVersion   uint64 `json:"acceptedVersion"`
}

type OpRejectPayload struct {
	ClientOpID         string       `json:"clientOpId"`
	Reason             string       `json:"reason"`
	AuthoritativeShape *model.Shape `json:"authoritativeShape,omitempty"`
}

type OpBroadcast struct {
	ServerSeq uint64          `json:"serverSeq"`
	AuthorID  string          `json:"authorId"`
	Kind      string          `json:"opKind"`
	TargetID  string          `json:"targetId"`
	Patch     json.RawMessage `json:"patch,omitempty"`
	Lamport   uint64          `json:"lamport"`
	Version   uint64          `json:"version"`
}

type MemberEvent struct {
	Member model.Member `json:"member"`
}

type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ResyncPayload struct {
	Reason    string          `json:"reason"`
	Snapshot  *model.Snapshot `json:"snapshot,omitempty"`
	ServerSeq uint64          `json:"serverSeq"`
}

type CreatePatch struct {
	Shape *model.Shape `json:"shape"`
}

type UpdatePatch map[string]any

type DeletePatch struct {
	IDs []string `json:"ids"`
}

type ReorderPatch struct {
	ID    string  `json:"id"`
	Z     float64 `json:"z"`
	Place string  `json:"place,omitempty"` // top|bottom|up|down
}

type GroupPatch struct {
	GroupID string   `json:"groupId"`
	IDs     []string `json:"ids"`
}

func Encode(typ, id string, payload any) ([]byte, error) {
	var raw json.RawMessage
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		raw = b
	}
	return json.Marshal(Envelope{V: ProtoVersion, Type: typ, ID: id, Payload: raw})
}

func Decode(data []byte) (Envelope, error) {
	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return env, err
	}
	return env, nil
}

func DecodePayload[T any](raw json.RawMessage) (T, error) {
	var v T
	if len(raw) == 0 {
		return v, nil
	}
	err := json.Unmarshal(raw, &v)
	return v, err
}

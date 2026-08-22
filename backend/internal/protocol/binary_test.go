package protocol

import (
	"encoding/json"
	"math"
	"testing"

	"gomiro/internal/model"
)

func TestCursorRoundTrip(t *testing.T) {
	raw := EncodeCursorC2S(12.5, -8.25)
	if len(raw) > 40 {
		t.Fatalf("c2s too large: %d", len(raw))
	}
	x, y, ok := DecodeCursorC2S(raw)
	if !ok || x != 12.5 || y != -8.25 {
		t.Fatalf("c2s decode %v %v %v", x, y, ok)
	}
	frame := EncodeCursorS2C([]CursorSample{{UserIdx: 7, X: 1, Y: 2}, {UserIdx: 9, X: 3, Y: 4}})
	if len(frame) > 40*2 {
		t.Fatalf("s2c large: %d", len(frame))
	}
	got, ok := DecodeCursorS2C(frame)
	if !ok || len(got) != 2 || got[1].UserIdx != 9 {
		t.Fatalf("s2c %+v ok=%v", got, ok)
	}
}

func TestCursorRejectsNaN(t *testing.T) {
	raw := EncodeCursorC2S(float32(math.NaN()), 1)
	if _, _, ok := DecodeCursorC2S(raw); ok {
		t.Fatal("nan should fail")
	}
}

func TestValidateJoinAndOp(t *testing.T) {
	if err := ValidateJoin(JoinPayload{BoardID: "b_abcdef", Nickname: "Ada", Color: "#aabbcc", ProtoVersion: 1}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateJoin(JoinPayload{BoardID: "??", Nickname: "Ada"}); err == nil {
		t.Fatal("bad board id")
	}
	if err := ValidateOp(OpPayload{ClientOpID: "op_abcdef", Kind: OpUpdate, TargetID: "shp_abcdef"}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateOp(OpPayload{ClientOpID: "x", Kind: "nope"}); err == nil {
		t.Fatal("bad op")
	}
}

func TestSanitizeCreateRejectsInf(t *testing.T) {
	s := model.NewShape(model.KindRect, "shp_ffffaabbcc")
	s.X = math.Inf(1)
	if err := SanitizeCreate(s); err == nil {
		t.Fatal("inf should fail")
	}
}

func TestSanitizeUpdateUnknownField(t *testing.T) {
	_, err := SanitizeUpdatePatch(json.RawMessage(`{"evil":1}`))
	if err == nil {
		t.Fatal("unknown field")
	}
}

func TestValidateInboundFuzzish(t *testing.T) {
	bads := [][]byte{
		[]byte(""),
		[]byte("{"),
		[]byte(`{"v":9,"type":"join"}`),
		[]byte(`{"v":1,"type":"nope"}`),
		[]byte{0x99, 0, 0},
		EncodeCursorC2S(float32(math.Inf(1)), 0),
	}
	for i, raw := range bads {
		if err := ValidateInbound(raw, i == 4 || i == 5, 1024); err == nil {
			t.Fatalf("case %d should fail", i)
		}
	}
	goodJoin, _ := json.Marshal(map[string]any{
		"v": 1, "type": "join",
		"payload": map[string]any{"boardId": "b_abcdef", "nickname": "Ada", "color": "#aabbcc", "protoVersion": 1},
	})
	if err := ValidateInbound(goodJoin, false, 4096); err != nil {
		t.Fatal(err)
	}
}

func TestEnvelopeCodec(t *testing.T) {
	b, err := Encode(TypePing, "m1", map[string]int{"n": 1})
	if err != nil {
		t.Fatal(err)
	}
	env, err := Decode(b)
	if err != nil || env.Type != TypePing {
		t.Fatalf("%+v %v", env, err)
	}
}

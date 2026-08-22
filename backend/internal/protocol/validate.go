package protocol

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"gomiro/internal/model"
)

type ErrorCode string

const (
	ErrBadJSON      ErrorCode = "bad_json"
	ErrBadType      ErrorCode = "bad_type"
	ErrBadVersion   ErrorCode = "bad_version"
	ErrBadField     ErrorCode = "bad_field"
	ErrTooLarge     ErrorCode = "too_large"
	ErrRateLimited  ErrorCode = "rate_limited"
)

func (e ErrorCode) String() string { return string(e) }

type ValidationError struct {
	Code    ErrorCode
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func Fail(code ErrorCode, msg string) error {
	return &ValidationError{Code: code, Message: msg}
}

func ValidateEnvelope(env Envelope, rawLen int, maxBytes int) error {
	if rawLen > maxBytes {
		return Fail(ErrTooLarge, "message exceeds max size")
	}
	if env.V != ProtoVersion {
		return Fail(ErrBadVersion, fmt.Sprintf("unsupported proto v=%d", env.V))
	}
	switch env.Type {
	case TypeJoin, TypeOp, TypeCursor, TypeSelection, TypePing:
	default:
		return Fail(ErrBadType, "unknown message type")
	}
	return nil
}

func ValidateJoin(p JoinPayload) error {
	if !model.ValidID(p.BoardID) {
		return Fail(ErrBadField, "boardId invalid")
	}
	if strings.TrimSpace(p.Nickname) == "" {
		return Fail(ErrBadField, "nickname required")
	}
	if utf8.RuneCountInString(p.Nickname) > MaxNicknameRunes {
		return Fail(ErrBadField, "nickname too long")
	}
	if p.Color != "" && !model.ValidColor(p.Color) {
		return Fail(ErrBadField, "color must be #RRGGBB")
	}
	if p.ProtoVersion != 0 && p.ProtoVersion != ProtoVersion {
		return Fail(ErrBadVersion, "protoVersion mismatch")
	}
	if utf8.RuneCountInString(p.Passcode) > 64 {
		return Fail(ErrBadField, "passcode too long")
	}
	return nil
}

func ValidateOp(p OpPayload) error {
	if !model.ValidID(p.ClientOpID) && !looseOpID(p.ClientOpID) {
		return Fail(ErrBadField, "clientOpId invalid")
	}
	if !KnownOpKind(p.Kind) {
		return Fail(ErrBadField, "unknown opKind")
	}
	if p.Kind != OpGroup && p.Kind != OpUngroup && p.Kind != OpDelete {
		if p.TargetID != "" && !model.ValidID(p.TargetID) {
			return Fail(ErrBadField, "targetId invalid")
		}
	}
	if len(p.Patch) > MaxJSONBytes {
		return Fail(ErrTooLarge, "patch too large")
	}
	return nil
}

func looseOpID(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 6 || len(s) > 80 {
		return false
	}
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-') {
			return false
		}
	}
	return true
}

func ValidateSelection(ids []string) error {
	if len(ids) > MaxSelection {
		return Fail(ErrTooLarge, "too many selected ids")
	}
	for _, id := range ids {
		if id != "" && !model.ValidID(id) {
			return Fail(ErrBadField, "selection id invalid")
		}
	}
	return nil
}

func SanitizeCreate(s *model.Shape) error {
	if s == nil {
		return Fail(ErrBadField, "shape required")
	}
	if !model.ValidID(s.ID) {
		return Fail(ErrBadField, "shape.id invalid")
	}
	if !model.KnownKind(s.Kind) {
		return Fail(ErrBadField, "unknown kind")
	}
	if err := sanitizeGeom(s); err != nil {
		return err
	}
	if err := sanitizeStyle(s); err != nil {
		return err
	}
	if err := sanitizePoints(s); err != nil {
		return err
	}
	if utf8.RuneCountInString(s.Text) > MaxTextRunes {
		return Fail(ErrBadField, "text too long")
	}
	if s.ImageURL != "" && !validImageRef(s.ImageURL) {
		return Fail(ErrBadField, "imageUrl invalid")
	}
	s.Deleted = false
	return nil
}

func SanitizeUpdatePatch(raw json.RawMessage) (map[string]any, error) {
	if len(raw) == 0 {
		return nil, Fail(ErrBadField, "empty patch")
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, Fail(ErrBadJSON, "patch is not an object")
	}
	if len(m) == 0 && m != nil {
		return nil, Fail(ErrBadField, "empty patch")
	}
	if len(m) > MaxPatchFields {
		return nil, Fail(ErrTooLarge, "too many patch fields")
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		if !isPatchField(k) {
			return nil, Fail(ErrBadField, "unknown patch field: "+k)
		}
		nv, err := coerceField(k, v)
		if err != nil {
			return nil, err
		}
		out[k] = nv
	}
	return out, nil
}

func isPatchField(k string) bool {
	for _, f := range model.PatchFields {
		if f == k {
			return true
		}
	}
	return false
}

func coerceField(k string, v any) (any, error) {
	switch k {
	case "x", "y", "w", "h", "rotation", "strokeW", "opacity", "z", "fontSize", "radius":
		f, ok := asFloat(v)
		if !ok || !model.Finite(f) {
			return nil, Fail(ErrBadField, k+" must be finite number")
		}
		if math.Abs(f) > MaxCoordAbs {
			return nil, Fail(ErrBadField, k+" out of range")
		}
		if k == "w" || k == "h" {
			if f < 0 || f > MaxSize {
				return nil, Fail(ErrBadField, k+" out of range")
			}
		}
		if k == "opacity" && (f < 0 || f > 1) {
			return nil, Fail(ErrBadField, "opacity must be 0..1")
		}
		if k == "strokeW" && (f < 0 || f > MaxStrokeW) {
			return nil, Fail(ErrBadField, "strokeW out of range")
		}
		if k == "fontSize" && (f < 0 || f > MaxFontSize) {
			return nil, Fail(ErrBadField, "fontSize out of range")
		}
		return f, nil
	case "stroke":
		s, ok := v.(string)
		if !ok || !model.ValidColor(s) {
			return nil, Fail(ErrBadField, k+" must be #RRGGBB")
		}
		return strings.ToLower(s), nil
	case "fill":
		s, ok := v.(string)
		if !ok {
			return nil, Fail(ErrBadField, "fill must be string")
		}
		if s == "transparent" {
			s = "#00000000"
		}
		if strings.HasPrefix(s, "#") && len(s) == 9 {
			if !model.ValidColor(s[:7]) {
				return nil, Fail(ErrBadField, "fill must be #RRGGBB or #RRGGBBAA")
			}
			return strings.ToLower(s), nil
		}
		if !model.ValidColor(s) {
			return nil, Fail(ErrBadField, "fill must be #RRGGBB")
		}
		return strings.ToLower(s), nil
	case "dash":
		s, ok := v.(string)
		if !ok || (s != "solid" && s != "dashed") {
			return nil, Fail(ErrBadField, "dash must be solid|dashed")
		}
		return s, nil
	case "text":
		s, ok := v.(string)
		if !ok {
			return nil, Fail(ErrBadField, "text must be string")
		}
		if utf8.RuneCountInString(s) > MaxTextRunes {
			return nil, Fail(ErrBadField, "text too long")
		}
		return s, nil
	case "align":
		s, ok := v.(string)
		if !ok || (s != "left" && s != "center" && s != "right") {
			return nil, Fail(ErrBadField, "align invalid")
		}
		return s, nil
	case "startId", "endId", "groupId", "startAnchor", "endAnchor":
		s, ok := v.(string)
		if !ok {
			return nil, Fail(ErrBadField, k+" must be string")
		}
		if s != "" && k != "startAnchor" && k != "endAnchor" && !model.ValidID(s) {
			return nil, Fail(ErrBadField, k+" invalid id")
		}
		return s, nil
	case "imageUrl":
		s, ok := v.(string)
		if !ok || !validImageRef(s) {
			return nil, Fail(ErrBadField, "imageUrl invalid")
		}
		return s, nil
	case "points":
		pts, err := coercePoints(v)
		if err != nil {
			return nil, err
		}
		return pts, nil
	default:
		return nil, Fail(ErrBadField, "unknown field")
	}
}

func coercePoints(v any) ([]model.Point, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, Fail(ErrBadField, "points invalid")
	}
	var pts []model.Point
	if err := json.Unmarshal(raw, &pts); err != nil {
		return nil, Fail(ErrBadField, "points invalid")
	}
	if len(pts) > MaxPoints {
		return nil, Fail(ErrTooLarge, "too many points")
	}
	for i, p := range pts {
		if !model.Finite(p.X) || !model.Finite(p.Y) || !model.Finite(p.P) {
			return nil, Fail(ErrBadField, "point not finite")
		}
		if math.Abs(p.X) > MaxCoordAbs || math.Abs(p.Y) > MaxCoordAbs {
			return nil, Fail(ErrBadField, "point out of range")
		}
		pts[i].P = clamp(p.P, 0, 2)
	}
	return pts, nil
}

func asFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func sanitizeGeom(s *model.Shape) error {
	if !model.Finite(s.X) || !model.Finite(s.Y) || !model.Finite(s.W) || !model.Finite(s.H) || !model.Finite(s.Rotation) || !model.Finite(s.Z) || !model.Finite(s.Radius) {
		return Fail(ErrBadField, "geometry not finite")
	}
	if math.Abs(s.X) > MaxCoordAbs || math.Abs(s.Y) > MaxCoordAbs {
		return Fail(ErrBadField, "coord out of range")
	}
	if s.W < 0 || s.W > MaxSize || s.H < 0 || s.H > MaxSize {
		return Fail(ErrBadField, "size out of range")
	}
	return nil
}

func sanitizeStyle(s *model.Shape) error {
	if s.Stroke == "" {
		s.Stroke = "#1a1916"
	}
	if s.Fill == "" || s.Fill == "transparent" {
		s.Fill = "#00000000"
	}
	if strings.HasPrefix(s.Fill, "#") && len(s.Fill) == 9 {
		if !model.ValidColor(s.Fill[:7]) {
			return Fail(ErrBadField, "fill invalid")
		}
	} else if !model.ValidColor(s.Fill) {
		return Fail(ErrBadField, "fill invalid")
	}
	if !model.ValidColor(s.Stroke) {
		return Fail(ErrBadField, "stroke invalid")
	}
	if !model.Finite(s.StrokeW) || s.StrokeW < 0 || s.StrokeW > MaxStrokeW {
		return Fail(ErrBadField, "strokeW invalid")
	}
	if s.Dash != "dashed" {
		s.Dash = "solid"
	}
	if !model.Finite(s.Opacity) {
		s.Opacity = 1
	}
	s.Opacity = clamp(s.Opacity, 0, 1)
	if s.Align != "center" && s.Align != "right" {
		s.Align = "left"
	}
	if !model.Finite(s.FontSize) || s.FontSize <= 0 {
		s.FontSize = 16
	}
	if s.FontSize > MaxFontSize {
		s.FontSize = MaxFontSize
	}
	return nil
}

func sanitizePoints(s *model.Shape) error {
	if len(s.Points) == 0 {
		return nil
	}
	if len(s.Points) > MaxPoints {
		return Fail(ErrTooLarge, "too many points")
	}
	for i, p := range s.Points {
		if !model.Finite(p.X) || !model.Finite(p.Y) || !model.Finite(p.P) {
			return Fail(ErrBadField, "point not finite")
		}
		s.Points[i].P = clamp(p.P, 0, 2)
	}
	return nil
}

func validImageRef(s string) bool {
	if s == "" {
		return true
	}
	if strings.HasPrefix(s, "/api/v1/files/") {
		hash := strings.TrimPrefix(s, "/api/v1/files/")
		if len(hash) == 64 {
			for _, c := range hash {
				if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
					return false
				}
			}
			return true
		}
	}
	return false
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func CheckFieldBounds(name string, n float64) error {
	spec, ok := model.LookupField(name)
	if !ok || spec.Kind != model.FieldNumber {
		return Fail(ErrBadField, "unknown numeric field")
	}
	if !model.Finite(n) {
		return Fail(ErrBadField, name+" not finite")
	}
	if n < spec.Min || n > spec.Max {
		return Fail(ErrBadField, name+" out of catalog range")
	}
	return nil
}

func CatalogNames() []string {
	out := make([]string, 0, len(model.FieldCatalog))
	for _, f := range model.FieldCatalog {
		out = append(out, f.Name)
	}
	return out
}

func KnownPatchField(name string) bool {
	_, ok := model.LookupField(name)
	return ok
}

func NormalizeDash(s string) string {
	if s == "dashed" {
		return "dashed"
	}
	return "solid"
}

func NormalizeAlign(s string) string {
	switch s {
	case "center", "right":
		return s
	default:
		return "left"
	}
}

func ShapeKindNeedsPoints(kind string) bool {
	return kind == model.KindFreedraw || kind == model.KindLine || kind == model.KindArrow
}

func EstimateJSONBytes(env Envelope) int {
	return 8 + len(env.Type) + len(env.ID) + len(env.Payload)
}

// ValidateInbound is the single entry used by the room loop and fuzz tests.
func ValidateInbound(raw []byte, isBin bool, maxBytes int) error {
	if len(raw) == 0 {
		return Fail(ErrBadJSON, "empty")
	}
	if len(raw) > maxBytes {
		return Fail(ErrTooLarge, "message exceeds max size")
	}
	if isBin {
		if raw[0] == BinCursorC2S {
			if _, _, ok := DecodeCursorC2S(raw); !ok {
				return Fail(ErrBadField, "bad cursor")
			}
			return nil
		}
		return Fail(ErrBadType, "unknown binary type")
	}
	env, err := Decode(raw)
	if err != nil {
		return Fail(ErrBadJSON, "malformed json")
	}
	if err := ValidateEnvelope(env, len(raw), maxBytes); err != nil {
		return err
	}
	switch env.Type {
	case TypeJoin:
		p, err := DecodePayload[JoinPayload](env.Payload)
		if err != nil {
			return Fail(ErrBadJSON, "bad join")
		}
		return ValidateJoin(p)
	case TypeOp:
		p, err := DecodePayload[OpPayload](env.Payload)
		if err != nil {
			return Fail(ErrBadJSON, "bad op")
		}
		if err := ValidateOp(p); err != nil {
			return err
		}
		if p.Kind == OpCreate {
			cp, err := DecodePayload[CreatePatch](p.Patch)
			if err != nil {
				return Fail(ErrBadJSON, "bad create patch")
			}
			return SanitizeCreate(cp.Shape)
		}
		if p.Kind == OpUpdate {
			_, err := SanitizeUpdatePatch(p.Patch)
			return err
		}
	case TypeSelection:
		p, err := DecodePayload[SelectionPayload](env.Payload)
		if err != nil {
			return Fail(ErrBadJSON, "bad selection")
		}
		return ValidateSelection(p.ShapeIDs)
	}
	return nil
}

package model

import (
	"encoding/json"
	"math"
	"strings"
)

const (
	KindRect     = "rect"
	KindEllipse  = "ellipse"
	KindDiamond  = "diamond"
	KindLine     = "line"
	KindArrow    = "arrow"
	KindFreedraw = "freedraw"
	KindText     = "text"
	KindSticky   = "sticky"
	KindImage    = "image"
)

var KnownKinds = map[string]struct{}{
	KindRect: {}, KindEllipse: {}, KindDiamond: {}, KindLine: {},
	KindArrow: {}, KindFreedraw: {}, KindText: {}, KindSticky: {}, KindImage: {},
}

// Patchable fields used by property-level LWW.
var PatchFields = []string{
	"x", "y", "w", "h", "rotation", "stroke", "fill", "strokeW", "dash",
	"opacity", "z", "text", "fontSize", "align", "points", "startId",
	"endId", "startAnchor", "endAnchor", "imageUrl", "groupId", "radius",
}

type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	P float64 `json:"p,omitempty"`
}

type Shape struct {
	ID          string             `json:"id"`
	Kind        string             `json:"kind"`
	X           float64            `json:"x"`
	Y           float64            `json:"y"`
	W           float64            `json:"w"`
	H           float64            `json:"h"`
	Rotation    float64            `json:"rotation"`
	Stroke      string             `json:"stroke"`
	Fill        string             `json:"fill"`
	StrokeW     float64            `json:"strokeW"`
	Dash        string             `json:"dash"`
	Opacity     float64            `json:"opacity"`
	Z           float64            `json:"z"`
	Text        string             `json:"text,omitempty"`
	FontSize    float64            `json:"fontSize,omitempty"`
	Align       string             `json:"align,omitempty"`
	Points      []Point            `json:"points,omitempty"`
	StartID     string             `json:"startId,omitempty"`
	EndID       string             `json:"endId,omitempty"`
	StartAnchor string             `json:"startAnchor,omitempty"`
	EndAnchor   string             `json:"endAnchor,omitempty"`
	ImageURL    string             `json:"imageUrl,omitempty"`
	GroupID     string             `json:"groupId,omitempty"`
	Radius      float64            `json:"radius,omitempty"`
	Deleted     bool               `json:"deleted,omitempty"`
	Version     uint64             `json:"version"`
	LastWriter  string             `json:"lastWriterId"`
	UpdatedAt   int64              `json:"updatedAt"`
	PropVer     map[string]uint64  `json:"propVer,omitempty"`
}

func NewShape(kind, id string) *Shape {
	return &Shape{
		ID:       id,
		Kind:     kind,
		W:        160,
		H:        96,
		Stroke:   "#1a1916",
		Fill:     "#f4efe6",
		StrokeW:  2,
		Dash:     "solid",
		Opacity:  1,
		FontSize: 16,
		Align:    "left",
		PropVer:  map[string]uint64{},
	}
}

func (s *Shape) Clone() *Shape {
	if s == nil {
		return nil
	}
	cp := *s
	if s.Points != nil {
		cp.Points = append([]Point(nil), s.Points...)
	}
	if s.PropVer != nil {
		cp.PropVer = make(map[string]uint64, len(s.PropVer))
		for k, v := range s.PropVer {
			cp.PropVer[k] = v
		}
	}
	return &cp
}

func (s *Shape) EnsurePropVer() {
	if s.PropVer == nil {
		s.PropVer = map[string]uint64{}
	}
}

func Finite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func KnownKind(k string) bool {
	_, ok := KnownKinds[strings.TrimSpace(k)]
	return ok
}

func (s *Shape) MarshalJSON() ([]byte, error) {
	type alias Shape
	return json.Marshal((*alias)(s))
}

func (s *Shape) GetNumber(field string) (float64, bool) {
	if s == nil {
		return 0, false
	}
	switch field {
	case "x":
		return s.X, true
	case "y":
		return s.Y, true
	case "w":
		return s.W, true
	case "h":
		return s.H, true
	case "rotation":
		return s.Rotation, true
	case "strokeW":
		return s.StrokeW, true
	case "opacity":
		return s.Opacity, true
	case "z":
		return s.Z, true
	case "fontSize":
		return s.FontSize, true
	case "radius":
		return s.Radius, true
	default:
		return 0, false
	}
}

func (s *Shape) GetString(field string) (string, bool) {
	if s == nil {
		return "", false
	}
	switch field {
	case "stroke":
		return s.Stroke, true
	case "fill":
		return s.Fill, true
	case "dash":
		return s.Dash, true
	case "text":
		return s.Text, true
	case "align":
		return s.Align, true
	case "startId":
		return s.StartID, true
	case "endId":
		return s.EndID, true
	case "startAnchor":
		return s.StartAnchor, true
	case "endAnchor":
		return s.EndAnchor, true
	case "imageUrl":
		return s.ImageURL, true
	case "groupId":
		return s.GroupID, true
	default:
		return "", false
	}
}

func (s *Shape) Diff(prev *Shape) map[string]any {
	if s == nil {
		return nil
	}
	out := map[string]any{}
	if prev == nil {
		out["x"], out["y"], out["w"], out["h"] = s.X, s.Y, s.W, s.H
		return out
	}
	if s.X != prev.X {
		out["x"] = s.X
	}
	if s.Y != prev.Y {
		out["y"] = s.Y
	}
	if s.W != prev.W {
		out["w"] = s.W
	}
	if s.H != prev.H {
		out["h"] = s.H
	}
	if s.Rotation != prev.Rotation {
		out["rotation"] = s.Rotation
	}
	if s.Stroke != prev.Stroke {
		out["stroke"] = s.Stroke
	}
	if s.Fill != prev.Fill {
		out["fill"] = s.Fill
	}
	if s.StrokeW != prev.StrokeW {
		out["strokeW"] = s.StrokeW
	}
	if s.Dash != prev.Dash {
		out["dash"] = s.Dash
	}
	if s.Opacity != prev.Opacity {
		out["opacity"] = s.Opacity
	}
	if s.Z != prev.Z {
		out["z"] = s.Z
	}
	if s.Text != prev.Text {
		out["text"] = s.Text
	}
	if s.FontSize != prev.FontSize {
		out["fontSize"] = s.FontSize
	}
	if s.Align != prev.Align {
		out["align"] = s.Align
	}
	if s.StartID != prev.StartID {
		out["startId"] = s.StartID
	}
	if s.EndID != prev.EndID {
		out["endId"] = s.EndID
	}
	if s.StartAnchor != prev.StartAnchor {
		out["startAnchor"] = s.StartAnchor
	}
	if s.EndAnchor != prev.EndAnchor {
		out["endAnchor"] = s.EndAnchor
	}
	if s.ImageURL != prev.ImageURL {
		out["imageUrl"] = s.ImageURL
	}
	if s.GroupID != prev.GroupID {
		out["groupId"] = s.GroupID
	}
	if s.Radius != prev.Radius {
		out["radius"] = s.Radius
	}
	return out
}

func (s *Shape) BoundsCenter() (float64, float64) {
	if s == nil {
		return 0, 0
	}
	return s.X + s.W/2, s.Y + s.H/2
}

type FieldKind int

const (
	FieldNumber FieldKind = iota
	FieldString
	FieldPoints
)

type FieldSpec struct {
	Name string
	Kind FieldKind
	Min  float64
	Max  float64
}

var FieldCatalog = []FieldSpec{
	{Name: "x", Kind: FieldNumber, Min: -1e7, Max: 1e7},
	{Name: "y", Kind: FieldNumber, Min: -1e7, Max: 1e7},
	{Name: "w", Kind: FieldNumber, Min: 0, Max: 1e6},
	{Name: "h", Kind: FieldNumber, Min: 0, Max: 1e6},
	{Name: "rotation", Kind: FieldNumber, Min: -1e7, Max: 1e7},
	{Name: "strokeW", Kind: FieldNumber, Min: 0, Max: 64},
	{Name: "opacity", Kind: FieldNumber, Min: 0, Max: 1},
	{Name: "z", Kind: FieldNumber, Min: -1e7, Max: 1e7},
	{Name: "fontSize", Kind: FieldNumber, Min: 0, Max: 400},
	{Name: "radius", Kind: FieldNumber, Min: 0, Max: 1e6},
	{Name: "stroke", Kind: FieldString},
	{Name: "fill", Kind: FieldString},
	{Name: "dash", Kind: FieldString},
	{Name: "text", Kind: FieldString},
	{Name: "align", Kind: FieldString},
	{Name: "startId", Kind: FieldString},
	{Name: "endId", Kind: FieldString},
	{Name: "startAnchor", Kind: FieldString},
	{Name: "endAnchor", Kind: FieldString},
	{Name: "imageUrl", Kind: FieldString},
	{Name: "groupId", Kind: FieldString},
	{Name: "points", Kind: FieldPoints},
}

func LookupField(name string) (FieldSpec, bool) {
	for _, f := range FieldCatalog {
		if f.Name == name {
			return f, true
		}
	}
	return FieldSpec{}, false
}

func DefaultFill(kind string) string {
	switch kind {
	case KindSticky:
		return "#f4e3b2"
	case KindText:
		return "#00000000"
	case KindLine, KindArrow, KindFreedraw:
		return "#00000000"
	default:
		return "#f4efe6"
	}
}

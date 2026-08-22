package engine

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"math"

	"gomiro/internal/model"
)

type AABB struct {
	MinX, MinY, MaxX, MaxY float64
}

func (a AABB) Intersects(b AABB) bool {
	return a.MinX <= b.MaxX && a.MaxX >= b.MinX && a.MinY <= b.MaxY && a.MaxY >= b.MinY
}

func (a AABB) Union(b AABB) AABB {
	return AABB{
		MinX: math.Min(a.MinX, b.MinX),
		MinY: math.Min(a.MinY, b.MinY),
		MaxX: math.Max(a.MaxX, b.MaxX),
		MaxY: math.Max(a.MaxY, b.MaxY),
	}
}

func (a AABB) Empty() bool {
	return a.MaxX < a.MinX || a.MaxY < a.MinY
}

func ShapeAABB(s *model.Shape) AABB {
	if s == nil {
		return AABB{1, 1, -1, -1}
	}
	if len(s.Points) > 0 && (s.Kind == model.KindFreedraw || s.Kind == model.KindLine || s.Kind == model.KindArrow) {
		minX, minY := s.Points[0].X, s.Points[0].Y
		maxX, maxY := minX, minY
		for _, p := range s.Points {
			if p.X < minX {
				minX = p.X
			}
			if p.Y < minY {
				minY = p.Y
			}
			if p.X > maxX {
				maxX = p.X
			}
			if p.Y > maxY {
				maxY = p.Y
			}
		}
		pad := s.StrokeW + 8
		return AABB{minX + s.X - pad, minY + s.Y - pad, maxX + s.X + pad, maxY + s.Y + pad}
	}
	x1, y1 := s.X, s.Y
	x2, y2 := s.X+s.W, s.Y+s.H
	if s.W < 0 {
		x1, x2 = x2, x1
	}
	if s.H < 0 {
		y1, y2 = y2, y1
	}
	if s.Rotation == 0 {
		return AABB{x1, y1, x2, y2}
	}
	cx := (x1 + x2) / 2
	cy := (y1 + y2) / 2
	rad := s.Rotation * math.Pi / 180
	cos, sin := math.Cos(rad), math.Sin(rad)
	corners := [4][2]float64{{x1, y1}, {x2, y1}, {x2, y2}, {x1, y2}}
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, c := range corners {
		dx, dy := c[0]-cx, c[1]-cy
		rx := cx + dx*cos - dy*sin
		ry := cy + dx*sin + dy*cos
		if rx < minX {
			minX = rx
		}
		if ry < minY {
			minY = ry
		}
		if rx > maxX {
			maxX = rx
		}
		if ry > maxY {
			maxY = ry
		}
	}
	return AABB{minX, minY, maxX, maxY}
}

func ContentBounds(shapes map[string]*model.Shape) AABB {
	var acc AABB
	first := true
	for _, s := range shapes {
		if s == nil || s.Deleted {
			continue
		}
		b := ShapeAABB(s)
		if first {
			acc = b
			first = false
			continue
		}
		acc = acc.Union(b)
	}
	if first {
		return AABB{-400, -300, 400, 300}
	}
	return acc
}

func AnchorPoint(s *model.Shape, name string) (float64, float64) {
	if s == nil {
		return 0, 0
	}
	cx, cy := s.X+s.W/2, s.Y+s.H/2
	switch name {
	case "n":
		return cx, s.Y
	case "s":
		return cx, s.Y + s.H
	case "w":
		return s.X, cy
	case "e":
		return s.X + s.W, cy
	case "nw":
		return s.X, s.Y
	case "ne":
		return s.X + s.W, s.Y
	case "sw":
		return s.X, s.Y + s.H
	case "se":
		return s.X + s.W, s.Y + s.H
	default:
		return cx, cy
	}
}

func Distance(ax, ay, bx, by float64) float64 {
	dx, dy := ax-bx, ay-by
	return math.Hypot(dx, dy)
}

func PointInAABB(x, y float64, b AABB) bool {
	return x >= b.MinX && x <= b.MaxX && y >= b.MinY && y <= b.MaxY
}

func PointInRotatedRect(x, y float64, s *model.Shape) bool {
	if s == nil {
		return false
	}
	if s.Rotation == 0 {
		return PointInAABB(x, y, ShapeAABB(s))
	}
	cx, cy := s.X+s.W/2, s.Y+s.H/2
	rad := -s.Rotation * math.Pi / 180
	cos, sin := math.Cos(rad), math.Sin(rad)
	dx, dy := x-cx, y-cy
	lx := dx*cos - dy*sin
	ly := dx*sin + dy*cos
	hw, hh := math.Abs(s.W)/2, math.Abs(s.H)/2
	return math.Abs(lx) <= hw && math.Abs(ly) <= hh
}

func PointInEllipse(x, y float64, s *model.Shape) bool {
	if s == nil || s.W == 0 || s.H == 0 {
		return false
	}
	cx, cy := s.X+s.W/2, s.Y+s.H/2
	nx := (x - cx) / (s.W / 2)
	ny := (y - cy) / (s.H / 2)
	return nx*nx+ny*ny <= 1
}

func PointInDiamond(x, y float64, s *model.Shape) bool {
	if s == nil || s.W == 0 || s.H == 0 {
		return false
	}
	cx, cy := s.X+s.W/2, s.Y+s.H/2
	return math.Abs((x-cx)/(s.W/2))+math.Abs((y-cy)/(s.H/2)) <= 1
}

func DistToSegment(px, py, ax, ay, bx, by float64) float64 {
	dx, dy := bx-ax, by-ay
	if dx == 0 && dy == 0 {
		return Distance(px, py, ax, ay)
	}
	t := ((px-ax)*dx + (py-ay)*dy) / (dx*dx + dy*dy)
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	return Distance(px, py, ax+t*dx, ay+t*dy)
}

func HitTest(s *model.Shape, x, y, pad float64) bool {
	if s == nil || s.Deleted {
		return false
	}
	switch s.Kind {
	case model.KindEllipse:
		return PointInEllipse(x, y, s)
	case model.KindDiamond:
		return PointInDiamond(x, y, s)
	case model.KindLine, model.KindArrow, model.KindFreedraw:
		return hitStroke(s, x, y, pad)
	default:
		return PointInRotatedRect(x, y, s)
	}
}

func hitStroke(s *model.Shape, x, y, pad float64) bool {
	tol := s.StrokeW/2 + pad
	if len(s.Points) >= 2 {
		for i := 1; i < len(s.Points); i++ {
			a, b := s.Points[i-1], s.Points[i]
			if DistToSegment(x, y, a.X+s.X, a.Y+s.Y, b.X+s.X, b.Y+s.Y) <= tol {
				return true
			}
		}
		return false
	}
	return DistToSegment(x, y, s.X, s.Y, s.X+s.W, s.Y+s.H) <= tol
}

func inflate(a AABB, pad float64) AABB {
	return AABB{a.MinX - pad, a.MinY - pad, a.MaxX + pad, a.MaxY + pad}
}

func NearestAnchor(s *model.Shape, x, y float64) string {
	if s == nil {
		return "c"
	}
	names := []string{"n", "s", "w", "e", "nw", "ne", "sw", "se", "c"}
	best, bestD := "c", math.Inf(1)
	for _, n := range names {
		ax, ay := AnchorPoint(s, n)
		d := Distance(x, y, ax, ay)
		if d < bestD {
			bestD = d
			best = n
		}
	}
	return best
}

func PickTop(shapes map[string]*model.Shape, x, y, pad float64) *model.Shape {
	ids := SortedIDs(shapes)
	for i := len(ids) - 1; i >= 0; i-- {
		s := shapes[ids[i]]
		if HitTest(s, x, y, pad) {
			return s
		}
	}
	return nil
}

func MarqueeHits(shapes map[string]*model.Shape, box AABB) []string {
	var out []string
	for _, s := range shapes {
		if s == nil || s.Deleted {
			continue
		}
		if ShapeAABB(s).Intersects(box) {
			out = append(out, s.ID)
		}
	}
	return out
}

const thumbW, thumbH = 320, 200

// ThumbnailPNG draws a schematic of the board into a 320×200 PNG data URL.
func ThumbnailPNG(shapes map[string]*model.Shape) string {
	world := ContentBounds(shapes)
	ww := world.MaxX - world.MinX
	wh := world.MaxY - world.MinY
	if ww < 1 {
		ww = 1
	}
	if wh < 1 {
		wh = 1
	}
	pad := 16.0
	sx := (float64(thumbW) - pad*2) / ww
	sy := (float64(thumbH) - pad*2) / wh
	scale := math.Min(sx, sy)
	img := image.NewRGBA(image.Rect(0, 0, thumbW, thumbH))
	bg := color.RGBA{0x16, 0x14, 0x11, 0xff}
	ink := color.RGBA{0xc4, 0xa3, 0x5a, 0xff}
	for y := 0; y < thumbH; y++ {
		for x := 0; x < thumbW; x++ {
			img.SetRGBA(x, y, bg)
		}
	}
	project := func(x, y float64) (int, int) {
		px := pad + (x-world.MinX)*scale
		py := pad + (y-world.MinY)*scale
		return int(px), int(py)
	}
	ids := SortedIDs(shapes)
	if len(ids) > 400 {
		ids = ids[len(ids)-400:]
	}
	for _, id := range ids {
		s := shapes[id]
		if s == nil {
			continue
		}
		b := ShapeAABB(s)
		x1, y1 := project(b.MinX, b.MinY)
		x2, y2 := project(b.MaxX, b.MaxY)
		strokeRect(img, x1, y1, x2, y2, ink)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

func strokeRect(img *image.RGBA, x1, y1, x2, y2 int, c color.RGBA) {
	if x2 < x1 {
		x1, x2 = x2, x1
	}
	if y2 < y1 {
		y1, y2 = y2, y1
	}
	if x1 < 0 {
		x1 = 0
	}
	if y1 < 0 {
		y1 = 0
	}
	if x2 >= thumbW {
		x2 = thumbW - 1
	}
	if y2 >= thumbH {
		y2 = thumbH - 1
	}
	for x := x1; x <= x2; x++ {
		img.Set(x, y1, c)
		img.Set(x, y2, c)
	}
	for y := y1; y <= y2; y++ {
		img.Set(x1, y, c)
		img.Set(x2, y, c)
	}
}


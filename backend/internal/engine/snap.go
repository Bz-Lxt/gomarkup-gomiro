package engine

import "math"

type Guide struct {
	Kind  string  `json:"kind"` // v | h
	At    float64 `json:"at"`
	From  float64 `json:"from"`
	To    float64 `json:"to"`
}

type SnapResult struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Guides []Guide `json:"guides"`
	Snapped bool   `json:"snapped"`
}

// SnapMove aligns the moving AABB to nearby edges/centers within threshold
// (world units). Used by the server only for optional "snap preview" RPC-less
// local computation; the same algorithm is mirrored on the client.
func SnapMove(moving AABB, others []AABB, threshold float64) SnapResult {
	if threshold <= 0 {
		threshold = 8
	}
	res := SnapResult{X: 0, Y: 0}
	bestX := threshold + 1
	bestY := threshold + 1
	mx := []float64{moving.MinX, (moving.MinX + moving.MaxX) / 2, moving.MaxX}
	my := []float64{moving.MinY, (moving.MinY + moving.MaxY) / 2, moving.MaxY}
	for _, o := range others {
		ox := []float64{o.MinX, (o.MinX + o.MaxX) / 2, o.MaxX}
		oy := []float64{o.MinY, (o.MinY + o.MaxY) / 2, o.MaxY}
		for _, a := range mx {
			for _, b := range ox {
				d := b - a
				ad := math.Abs(d)
				if ad < bestX && ad <= threshold {
					bestX = ad
					res.X = d
					res.Guides = replaceGuide(res.Guides, "v", b, math.Min(moving.MinY, o.MinY), math.Max(moving.MaxY, o.MaxY))
				}
			}
		}
		for _, a := range my {
			for _, b := range oy {
				d := b - a
				ad := math.Abs(d)
				if ad < bestY && ad <= threshold {
					bestY = ad
					res.Y = d
					res.Guides = replaceGuide(res.Guides, "h", b, math.Min(moving.MinX, o.MinX), math.Max(moving.MaxX, o.MaxX))
				}
			}
		}
	}
	res.Snapped = bestX <= threshold || bestY <= threshold
	if bestX > threshold {
		res.X = 0
	}
	if bestY > threshold {
		res.Y = 0
	}
	return res
}

func replaceGuide(in []Guide, kind string, at, from, to float64) []Guide {
	out := in[:0]
	for _, g := range in {
		if g.Kind != kind {
			out = append(out, g)
		}
	}
	return append(out, Guide{Kind: kind, At: at, From: from, To: to})
}

func CollectAABBs(shapes map[string]*struct {
	MinX, MinY, MaxX, MaxY float64
	ID                     string
}, skip string) []AABB {
	out := make([]AABB, 0, len(shapes))
	for id, s := range shapes {
		if id == skip || s == nil {
			continue
		}
		out = append(out, AABB{s.MinX, s.MinY, s.MaxX, s.MaxY})
	}
	return out
}

func SnapThreshold(scale float64) float64 {
	if scale <= 0 {
		return 8
	}
	t := 8 / scale
	if t < 2 {
		return 2
	}
	if t > 24 {
		return 24
	}
	return t
}

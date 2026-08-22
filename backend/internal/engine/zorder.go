package engine

import (
	"math"
	"sort"

	"gomiro/internal/model"
)

const zStep = 1.0
const zEps = 1e-6

func SortedIDs(shapes map[string]*model.Shape) []string {
	ids := make([]string, 0, len(shapes))
	for id, s := range shapes {
		if s != nil && !s.Deleted {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool {
		a, b := shapes[ids[i]], shapes[ids[j]]
		if a.Z == b.Z {
			return a.ID < b.ID
		}
		return a.Z < b.Z
	})
	return ids
}

func NextZ(shapes map[string]*model.Shape) float64 {
	max := 0.0
	for _, s := range shapes {
		if s != nil && !s.Deleted && s.Z > max {
			max = s.Z
		}
	}
	return max + zStep
}

func RecomputeZ(shapes map[string]*model.Shape, id, place string) (float64, bool) {
	s := shapes[id]
	if s == nil || s.Deleted {
		return 0, false
	}
	ids := SortedIDs(shapes)
	if len(ids) == 0 {
		return zStep, true
	}
	idx := -1
	for i, x := range ids {
		if x == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return NextZ(shapes), true
	}
	switch place {
	case "top":
		return shapes[ids[len(ids)-1]].Z + zStep, true
	case "bottom":
		return shapes[ids[0]].Z - zStep, true
	case "up":
		if idx >= len(ids)-1 {
			return s.Z, true
		}
		hi := shapes[ids[idx+1]].Z
		var next float64
		if idx+2 < len(ids) {
			next = shapes[ids[idx+2]].Z
			return mid(hi, next), true
		}
		return hi + zStep, true
	case "down":
		if idx <= 0 {
			return s.Z, true
		}
		lo := shapes[ids[idx-1]].Z
		if idx-2 >= 0 {
			prev := shapes[ids[idx-2]].Z
			return mid(prev, lo), true
		}
		return lo - zStep, true
	default:
		return s.Z, true
	}
}

func mid(a, b float64) float64 {
	m := (a + b) / 2
	if math.Abs(b-a) < zEps {
		return a + zStep/1000
	}
	return m
}

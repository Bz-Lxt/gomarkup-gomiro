package engine

import (
	"testing"

	"gomiro/internal/model"
)

func TestHitRectAndDiamond(t *testing.T) {
	r := model.NewShape(model.KindRect, "shp_hitrect01")
	r.X, r.Y, r.W, r.H = 0, 0, 100, 50
	if !HitTest(r, 10, 10, 0) || HitTest(r, 400, 400, 0) {
		t.Fatal("rect hit")
	}
	d := model.NewShape(model.KindDiamond, "shp_hitdia001")
	d.X, d.Y, d.W, d.H = 0, 0, 100, 100
	if !PointInDiamond(50, 50, d) || PointInDiamond(0, 0, d) {
		t.Fatal("diamond")
	}
}

func TestSnapMove(t *testing.T) {
	moving := AABB{0, 0, 40, 40}
	others := []AABB{{42, 0, 80, 40}}
	res := SnapMove(moving, others, 8)
	if !res.Snapped || res.X != 2 {
		t.Fatalf("snap %+v", res)
	}
}

func TestPickTopZ(t *testing.T) {
	a := model.NewShape(model.KindRect, "shp_zaaaaaaaa")
	b := model.NewShape(model.KindRect, "shp_zbbbbbbbb")
	a.X, a.Y, a.W, a.H, a.Z = 0, 0, 50, 50, 1
	b.X, b.Y, b.W, b.H, b.Z = 0, 0, 50, 50, 2
	got := PickTop(map[string]*model.Shape{a.ID: a, b.ID: b}, 10, 10, 0)
	if got == nil || got.ID != b.ID {
		t.Fatalf("want top b, got %+v", got)
	}
}

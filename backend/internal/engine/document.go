package engine

import (
	"encoding/json"
	"math"
	"sync"

	"gomiro/internal/model"
	"gomiro/internal/timeutil"
)

// Document is the in-memory authoritative canvas. All mutations must happen
// on the room goroutine (or under mu for snapshot cloning).
type Document struct {
	mu        sync.RWMutex
	BoardID   string
	ServerSeq uint64
	Shapes    map[string]*model.Shape
	Groups    map[string][]string
	SeenOps   map[string]uint64 // clientOpId -> serverSeq
	Dirty     bool
}

func NewDocument(boardID string) *Document {
	return &Document{
		BoardID: boardID,
		Shapes:  map[string]*model.Shape{},
		Groups:  map[string][]string{},
		SeenOps: map[string]uint64{},
	}
}

func (d *Document) Snapshot() *model.Snapshot {
	d.mu.RLock()
	defer d.mu.RUnlock()
	shapes := make(map[string]*model.Shape, len(d.Shapes))
	for id, s := range d.Shapes {
		if s == nil || s.Deleted {
			continue
		}
		shapes[id] = s.Clone()
	}
	groups := make(map[string][]string, len(d.Groups))
	for gid, ids := range d.Groups {
		groups[gid] = append([]string(nil), ids...)
	}
	return &model.Snapshot{
		BoardID:   d.BoardID,
		ServerSeq: d.ServerSeq,
		Shapes:    shapes,
		Groups:    groups,
	}
}

func (d *Document) Restore(snap *model.Snapshot) {
	d.mu.Lock()
	d.Shapes = map[string]*model.Shape{}
	d.Groups = map[string][]string{}
	if snap == nil {
		d.ServerSeq = 0
		d.Dirty = false
		return
	}
	defer d.mu.Unlock()
	d.ServerSeq = snap.ServerSeq
	for id, s := range snap.Shapes {
		if s != nil {
			d.Shapes[id] = s.Clone()
		}
	}
	for gid, ids := range snap.Groups {
		d.Groups[gid] = append([]string(nil), ids...)
	}
	d.Dirty = false
}

func (d *Document) Get(id string) *model.Shape {
	d.mu.RLock()
	defer d.mu.RUnlock()
	s := d.Shapes[id]
	if s == nil {
		return nil
	}
	return s.Clone()
}

func (d *Document) put(s *model.Shape) {
	d.Shapes[s.ID] = s
	d.Dirty = true
}

func (d *Document) nextSeqLocked() uint64 {
	d.ServerSeq++
	return d.ServerSeq
}

func (d *Document) MarkSeen(clientOpID string, seq uint64) {
	if clientOpID == "" {
		return
	}
	if len(d.SeenOps) > 20000 {
		// crude bound: drop half of the map
		i := 0
		for k := range d.SeenOps {
			delete(d.SeenOps, k)
			i++
			if i > 10000 {
				break
			}
		}
	}
	d.SeenOps[clientOpID] = seq
}

func (d *Document) Seen(clientOpID string) (uint64, bool) {
	seq, ok := d.SeenOps[clientOpID]
	return seq, ok
}

func (d *Document) Touch(writer string, seq uint64, s *model.Shape, fields []string) {
	s.EnsurePropVer()
	s.Version++
	s.LastWriter = writer
	s.UpdatedAt = timeutil.UnixMilli()
	for _, f := range fields {
		s.PropVer[f] = seq
	}
}

func (d *Document) MarshalShapes() ([]byte, error) {
	snap := d.Snapshot()
	return json.Marshal(snap)
}

func UnmarshalSnapshot(raw []byte) (*model.Snapshot, error) {
	var s model.Snapshot
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	if s.Shapes == nil {
		s.Shapes = map[string]*model.Shape{}
	}
	return &s, nil
}

func (d *Document) LiveCount() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	n := 0
	for _, s := range d.Shapes {
		if s != nil && !s.Deleted {
			n++
		}
	}
	return n
}

const gridCell = 256.0

func cellKey(cx, cy int) int64 {
	return (int64(cx) << 32) ^ int64(uint32(cy))
}

func cellsFor(b AABB) []int64 {
	if b.Empty() {
		return nil
	}
	x0 := int(math.Floor(b.MinX / gridCell))
	y0 := int(math.Floor(b.MinY / gridCell))
	x1 := int(math.Floor(b.MaxX / gridCell))
	y1 := int(math.Floor(b.MaxY / gridCell))
	if x1-x0 > 64 || y1-y0 > 64 {
		return []int64{cellKey(x0, y0)}
	}
	out := make([]int64, 0, (x1-x0+1)*(y1-y0+1))
	for x := x0; x <= x1; x++ {
		for y := y0; y <= y1; y++ {
			out = append(out, cellKey(x, y))
		}
	}
	return out
}

// QueryViewport returns live shapes whose AABB intersects the viewport.
// Used when a joining client reports lastSeq=0 but we still want to avoid
// shipping far-away ink on enormous boards (the full snapshot remains
// authoritative; this is an optional culling helper).
func (d *Document) QueryViewport(view AABB, limit int) []*model.Shape {
	if limit <= 0 {
		limit = 4000
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make([]*model.Shape, 0, 64)
	for _, s := range d.Shapes {
		if s == nil || s.Deleted {
			continue
		}
		if !ShapeAABB(s).Intersects(view) {
			continue
		}
		out = append(out, s.Clone())
		if len(out) >= limit {
			break
		}
	}
	return out
}

func (d *Document) BuildGrid() map[int64][]string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	grid := make(map[int64][]string, len(d.Shapes)/2+1)
	for id, s := range d.Shapes {
		if s == nil || s.Deleted {
			continue
		}
		for _, k := range cellsFor(ShapeAABB(s)) {
			grid[k] = append(grid[k], id)
		}
	}
	return grid
}

func QueryGrid(grid map[int64][]string, shapes map[string]*model.Shape, view AABB) []*model.Shape {
	seen := map[string]struct{}{}
	var out []*model.Shape
	for _, k := range cellsFor(view) {
		for _, id := range grid[k] {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			s := shapes[id]
			if s == nil || s.Deleted {
				continue
			}
			if ShapeAABB(s).Intersects(view) {
				out = append(out, s)
			}
		}
	}
	return out
}

func (d *Document) ThumbnailHint() map[string]any {
	b := ContentBounds(d.Shapes)
	return map[string]any{
		"minX": b.MinX, "minY": b.MinY, "maxX": b.MaxX, "maxY": b.MaxY,
		"count": d.LiveCount(), "seq": d.ServerSeq,
	}
}

func (d *Document) MissingSeq(from, to uint64) bool {
	return to > from+1
}

func (d *Document) CloneSeen() map[string]uint64 {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make(map[string]uint64, len(d.SeenOps))
	for k, v := range d.SeenOps {
		out[k] = v
	}
	return out
}

func (d *Document) RestoreSeen(m map[string]uint64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.SeenOps = map[string]uint64{}
	for k, v := range m {
		d.SeenOps[k] = v
	}
}

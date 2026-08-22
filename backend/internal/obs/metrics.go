package obs

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type Metrics struct {
	Connections    atomic.Int64
	Rooms          atomic.Int64
	OpsTotal       atomic.Int64
	OpsRejected    atomic.Int64
	CursorsDrop    atomic.Int64
	SlowDisconnect atomic.Int64
	BroadcastNs    atomic.Int64
	BroadcastN     atomic.Int64
	mu             sync.Mutex
	hist           [6]int64
}

func (m *Metrics) ObserveBroadcast(d time.Duration) {
	ms := d.Milliseconds()
	m.BroadcastNs.Add(d.Nanoseconds())
	m.BroadcastN.Add(1)
	idx := 5
	switch {
	case ms < 10:
		idx = 0
	case ms < 25:
		idx = 1
	case ms < 50:
		idx = 2
	case ms < 100:
		idx = 3
	case ms < 200:
		idx = 4
	}
	m.mu.Lock()
	m.hist[idx]++
	m.mu.Unlock()
}

func (m *Metrics) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		n := m.BroadcastN.Load()
		avg := 0.0
		if n > 0 {
			avg = float64(m.BroadcastNs.Load()) / float64(n) / 1e6
		}
		m.mu.Lock()
		h := m.hist
		m.mu.Unlock()
		fmt.Fprintf(w, "gomiro_connections %d\n", m.Connections.Load())
		fmt.Fprintf(w, "gomiro_rooms %d\n", m.Rooms.Load())
		fmt.Fprintf(w, "gomiro_ops_total %d\n", m.OpsTotal.Load())
		fmt.Fprintf(w, "gomiro_ops_rejected %d\n", m.OpsRejected.Load())
		fmt.Fprintf(w, "gomiro_cursors_dropped %d\n", m.CursorsDrop.Load())
		fmt.Fprintf(w, "gomiro_slow_disconnects %d\n", m.SlowDisconnect.Load())
		fmt.Fprintf(w, "gomiro_broadcast_avg_ms %.3f\n", avg)
		fmt.Fprintf(w, "gomiro_broadcast_le_10ms %d\n", h[0])
		fmt.Fprintf(w, "gomiro_broadcast_le_25ms %d\n", h[1])
		fmt.Fprintf(w, "gomiro_broadcast_le_50ms %d\n", h[2])
		fmt.Fprintf(w, "gomiro_broadcast_le_100ms %d\n", h[3])
		fmt.Fprintf(w, "gomiro_broadcast_le_200ms %d\n", h[4])
		fmt.Fprintf(w, "gomiro_broadcast_gt_200ms %d\n", h[5])
	})
}

package obs

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// Metrics holds process-wide observability counters.
//
// The plain integer counters (Connections, Rooms, ...) are updated with
// atomic operations and are safe to read concurrently.
//
// The broadcast-latency state (count, cumulative nanoseconds and the six
// latency buckets) is grouped under mu: every ObserveBroadcast update and
// every /metrics scrape operate on a single consistent snapshot taken while
// holding the lock. This guarantees that:
//
//   - the six buckets observed in one scrape sum exactly to the reported
//     broadcast total for that scrape, and
//   - no scrape reads a bucket while a concurrent ObserveBroadcast is
//     mutating it (the race detector reports a data race otherwise).
//
// Reading these values is only safe while holding mu; use snapshotBroadcast.
type Metrics struct {
	Connections    atomic.Int64
	Rooms          atomic.Int64
	OpsTotal       atomic.Int64
	OpsRejected    atomic.Int64
	CursorsDrop    atomic.Int64
	SlowDisconnect atomic.Int64

	mu          sync.Mutex
	broadcastN  int64
	broadcastNs int64
	hist        [6]int64
}

// ObserveBroadcast records a single broadcast latency sample. The count,
// cumulative nanoseconds and latency bucket are updated together under mu so
// that a concurrent /metrics scrape always sees a consistent view.
func (m *Metrics) ObserveBroadcast(d time.Duration) {
	ms := d.Milliseconds()
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
	m.broadcastN++
	m.broadcastNs += d.Nanoseconds()
	m.hist[idx]++
	m.mu.Unlock()
}

// broadcastSnapshot is a consistent point-in-time copy of the broadcast
// latency state, captured atomically under mu.
type broadcastSnapshot struct {
	n    int64
	ns   int64
	hist [6]int64
}

// snapshotBroadcast returns a consistent snapshot of the broadcast latency
// metrics. The returned struct is a value copy, so callers may use it freely
// after release without racing concurrent writers.
func (m *Metrics) snapshotBroadcast() broadcastSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	return broadcastSnapshot{
		n:    m.broadcastN,
		ns:   m.broadcastNs,
		hist: m.hist, // array value copy
	}
}

// Handler returns the Prometheus text exposition handler for /metrics.
// It takes a single atomic snapshot of the broadcast-latency state, then
// formats it outside the lock; ongoing ObserveBroadcast calls never touch
// the values being emitted.
func (m *Metrics) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		snap := m.snapshotBroadcast()
		avg := 0.0
		if snap.n > 0 {
			avg = float64(snap.ns) / float64(snap.n) / 1e6
		}
		fmt.Fprintf(w, "gomiro_connections %d\n", m.Connections.Load())
		fmt.Fprintf(w, "gomiro_rooms %d\n", m.Rooms.Load())
		fmt.Fprintf(w, "gomiro_ops_total %d\n", m.OpsTotal.Load())
		fmt.Fprintf(w, "gomiro_ops_rejected %d\n", m.OpsRejected.Load())
		fmt.Fprintf(w, "gomiro_cursors_dropped %d\n", m.CursorsDrop.Load())
		fmt.Fprintf(w, "gomiro_slow_disconnects %d\n", m.SlowDisconnect.Load())
		fmt.Fprintf(w, "gomiro_broadcast_total %d\n", snap.n)
		fmt.Fprintf(w, "gomiro_broadcast_avg_ms %.3f\n", avg)
		fmt.Fprintf(w, "gomiro_broadcast_le_10ms %d\n", snap.hist[0])
		fmt.Fprintf(w, "gomiro_broadcast_le_25ms %d\n", snap.hist[1])
		fmt.Fprintf(w, "gomiro_broadcast_le_50ms %d\n", snap.hist[2])
		fmt.Fprintf(w, "gomiro_broadcast_le_100ms %d\n", snap.hist[3])
		fmt.Fprintf(w, "gomiro_broadcast_le_200ms %d\n", snap.hist[4])
		fmt.Fprintf(w, "gomiro_broadcast_gt_200ms %d\n", snap.hist[5])
	})
}

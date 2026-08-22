package obs_test

import (
	"bufio"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"gomiro/internal/obs"
)

func TestMetricsScrapeConcurrentWithObservation(t *testing.T) {
	t.Parallel()

	metrics := &obs.Metrics{}
	handler := metrics.Handler()
	durations := []time.Duration{
		5 * time.Millisecond,
		15 * time.Millisecond,
		30 * time.Millisecond,
		75 * time.Millisecond,
		150 * time.Millisecond,
		250 * time.Millisecond,
	}

	const (
		writers       = 4
		writesPerLoop = 3000
		readers       = 4
		scrapes       = 500
	)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for worker := 0; worker < writers; worker++ {
		wg.Add(1)
		go func(offset int) {
			defer wg.Done()
			<-start
			for i := 0; i < writesPerLoop; i++ {
				metrics.ObserveBroadcast(durations[(i+offset)%len(durations)])
			}
		}(worker)
	}
	for worker := 0; worker < readers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < scrapes; i++ {
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
				if rec.Code != 200 {
					t.Errorf("metrics status = %d, want 200", rec.Code)
					return
				}
				if !strings.Contains(rec.Body.String(), "gomiro_broadcast_gt_200ms ") {
					t.Error("metrics response is missing the broadcast histogram")
					return
				}
			}
		}()
	}
	close(start)
	wg.Wait()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	values := metricValues(t, rec.Body.String())
	wantTotal := int64(writers * writesPerLoop)
	for _, name := range []string{
		"gomiro_broadcast_le_10ms",
		"gomiro_broadcast_le_25ms",
		"gomiro_broadcast_le_50ms",
		"gomiro_broadcast_le_100ms",
		"gomiro_broadcast_le_200ms",
		"gomiro_broadcast_gt_200ms",
	} {
		want := wantTotal / int64(len(durations))
		if values[name] != want {
			t.Errorf("%s = %d, want %d", name, values[name], want)
		}
	}
}

func metricValues(t *testing.T, body string) map[string]int64 {
	t.Helper()
	values := make(map[string]int64)
	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 {
			continue
		}
		value, err := strconv.ParseInt(fields[1], 10, 64)
		if err == nil {
			values[fields[0]] = value
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan metrics: %v", err)
	}
	return values
}

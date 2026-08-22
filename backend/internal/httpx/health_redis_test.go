package httpx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"gomiro/internal/cluster"
	"gomiro/internal/httpx"
)

func TestReadyReportsRedisFailure(t *testing.T) {
	bus := cluster.New("redis-address-without-port", "", "node_ready_test")
	defer bus.Close()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	httpx.Health{Bus: bus}.Ready(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusServiceUnavailable, rec.Body.String())
	}
	var body struct {
		Redis bool `json:"redis"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Redis {
		t.Fatalf("ready response reported redis=true after Redis PING failed")
	}
}

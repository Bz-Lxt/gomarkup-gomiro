package httpx

import (
	"context"
	"net/http"
	"time"

	"gomiro/internal/cluster"
	"gomiro/internal/store"
	"gomiro/internal/timeutil"
)

type Health struct {
	DB  *store.DB
	Bus *cluster.Bus
}

func (h Health) Live(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"time":   timeutil.Format(timeutil.Now()),
	})
}

func (h Health) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	dbOK := h.DB != nil && h.DB.Ping(ctx) == nil
	redisOK := h.Bus != nil && h.Bus.Ping(ctx) == nil
	code := http.StatusOK
	status := "ok"
	if !dbOK || !redisOK {
		code = http.StatusServiceUnavailable
		status = "degraded"
	}
	writeJSON(w, code, map[string]any{
		"status": status,
		"db":     dbOK,
		"redis":  redisOK,
		"time":   timeutil.Format(timeutil.Now()),
	})
}

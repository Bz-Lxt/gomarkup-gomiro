package httpx

import (
	"context"
	"errors"
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

	// Probe the dependencies. If the upstream request (or the 2s timeout) is
	// cancelled while a probe is in flight, the driver may still surface a
	// "dial tcp ..." error. We must NOT treat that as an outage, so we sniff
	// out cancellation first and, in that case, report the dependency as
	// "unknown" rather than degraded. A 503 is reserved for real failures.
	dbErr := error(nil)
	if h.DB == nil {
		dbErr = errors.New("db disabled")
	} else {
		dbErr = h.DB.Ping(ctx)
	}
	dbOK := dbErr == nil

	var redisErr error
	if h.Bus == nil {
		redisErr = cluster.ErrRedisDisabled()
	} else {
		redisErr = h.Bus.Ping(ctx)
	}
	redisOK := redisErr == nil

	dbCanceled := errors.Is(dbErr, context.Canceled) || errors.Is(dbErr, context.DeadlineExceeded)
	redisCanceled := cluster.IsCanceled(redisErr)

	code := http.StatusOK
	status := "ok"
	if !dbOK && !dbCanceled || !redisOK && !redisCanceled {
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

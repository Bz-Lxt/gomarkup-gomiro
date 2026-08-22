package httpx

import (
	"net/http"

	"gomiro/internal/cluster"
	"gomiro/internal/config"
	"gomiro/internal/obs"
	"gomiro/internal/store"
	"gomiro/internal/ws"
)

type Deps struct {
	Cfg config.Config
	DB  *store.DB
	Bus *cluster.Bus
	Hub *ws.Hub
	Met *obs.Metrics
}

func NewMux(d Deps) http.Handler {
	mux := http.NewServeMux()
	health := Health{DB: d.DB, Bus: d.Bus}
	boards := BoardAPI{DB: d.DB, Cfg: d.Cfg}
	up := UploadAPI{DB: d.DB, Cfg: d.Cfg}

	mux.HandleFunc("GET /healthz", health.Live)
	mux.HandleFunc("GET /readyz", health.Ready)
	mux.Handle("GET /metrics", d.Met.Handler())

	mux.HandleFunc("GET /api/v1/boards", boards.List)
	mux.HandleFunc("POST /api/v1/boards", boards.Create)
	mux.HandleFunc("GET /api/v1/boards/{id}", boards.Get)
	mux.HandleFunc("PATCH /api/v1/boards/{id}", boards.Patch)
	mux.HandleFunc("DELETE /api/v1/boards/{id}", boards.Delete)
	mux.HandleFunc("POST /api/v1/boards/{id}/unlock", boards.Unlock)

	mux.HandleFunc("POST /api/v1/uploads", up.Post)
	mux.HandleFunc("GET /api/v1/files/{hash}", up.Get)

	mux.HandleFunc("GET /ws", d.Hub.ServeWS)

	return Recover(CORS(RequestID(AccessLog(mux))))
}

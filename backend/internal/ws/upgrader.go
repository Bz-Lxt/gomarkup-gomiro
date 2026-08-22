package ws

import (
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
	"gomiro/internal/config"
)

func NewUpgrader(cfg config.Config) websocket.Upgrader {
	origins := map[string]struct{}{}
	allowAll := cfg.WSOrigin == "*" || cfg.WSOrigin == ""
	for _, o := range strings.Split(cfg.WSOrigin, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			origins[o] = struct{}{}
		}
	}
	return websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		CheckOrigin: func(r *http.Request) bool {
			if allowAll {
				return true
			}
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true
			}
			_, ok := origins[origin]
			return ok
		},
	}
}

package main

import (
	"context"
	"flag"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"gomiro/internal/cluster"
	"gomiro/internal/config"
	"gomiro/internal/httpx"
	"gomiro/internal/logx"
	"gomiro/internal/obs"
	"gomiro/internal/store"
	"gomiro/internal/ws"
)

func main() {
	healthcheck := flag.Bool("healthcheck", false, "probe local /healthz and exit")
	flag.Parse()
	if *healthcheck {
		os.Exit(probe())
	}

	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	if err := cfg.Validate(); err != nil {
		panic(err)
	}
	logx.Init(cfg.LogLevel, os.Stdout)
	log := logx.L()
	log.Info("boot", "cfg", cfg.Redacted())

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := os.MkdirAll(cfg.UploadDir, 0o755); err != nil {
		log.Error("upload dir", "err", err)
		os.Exit(1)
	}

	db, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("db", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	mig := cfg.MigrationsDir
	if !filepath.IsAbs(mig) {
		if _, err := os.Stat(mig); err != nil {
			if alt := "/app/migrations"; dirExists(alt) {
				mig = alt
			}
		}
	}
	if err := db.Migrate(ctx, mig); err != nil {
		log.Error("migrate", "err", err)
		os.Exit(1)
	}

	bus := cluster.New(cfg.RedisAddr, cfg.RedisPassword, cfg.NodeID)
	if err := bus.Ping(ctx); err != nil {
		// During shutdown the root context may already be cancelled. In that
		// case the ping error is a side-effect of cancellation, not a Redis
		// outage, so we exit quietly instead of logging "redis" as a failure.
		if cluster.IsCanceled(err) || (ctx != nil && ctx.Err() != nil) {
			log.Info("shutdown during redis ping")
			os.Exit(0)
		}
		log.Error("redis", "err", err)
		os.Exit(1)
	}
	bus.MarkReady(true)
	defer bus.Close()

	met := &obs.Metrics{}
	hub := ws.NewHub(cfg, db, bus, met)
	mux := httpx.NewMux(httpx.Deps{Cfg: cfg, DB: db, Bus: bus, Hub: hub, Met: met})

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       90 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}

	sweep := time.NewTicker(10 * time.Second)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-sweep.C:
				hub.Sweep()
			}
		}
	}()

	go func() {
		log.Info("listen", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("http", "err", err)
			cancel()
		}
	}()

	<-ctx.Done()
	sweep.Stop()
	log.Info("shutdown")
	shutCtx, done := context.WithTimeout(context.Background(), 12*time.Second)
	defer done()
	hub.Shutdown(shutCtx)
	_ = srv.Shutdown(shutCtx)
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func probe() int {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	host := "127.0.0.1"
	if len(addr) > 0 && addr[0] != ':' {
		host = addr
	} else {
		host = "127.0.0.1" + addr
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://" + host + "/healthz")
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return 1
	}
	return 0
}

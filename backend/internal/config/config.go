package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr            string
	NodeID              string
	DatabaseURL         string
	RedisAddr           string
	RedisPassword       string
	UploadDir           string
	MaxUploadBytes      int64
	RoomIdle            time.Duration
	SnapshotInterval    time.Duration
	EmptyRoomGrace      time.Duration
	LogLevel            string
	WSOrigin            string
	ProtoVersion        int
	MaxOpPerSec         int
	MaxMsgBytes         int
	PingInterval        time.Duration
	PongWait            time.Duration
	WriteWait           time.Duration
	SendQueueSize       int
	CursorTick          time.Duration
	BcryptCost          int
	MigrationsDir       string
	MetricsNamespace    string
}

func Load() (Config, error) {
	c := Config{
		HTTPAddr:         env("HTTP_ADDR", ":8080"),
		NodeID:           env("NODE_ID", hostnameFallback()),
		DatabaseURL:      env("DATABASE_URL", "postgres://gomiro:gomiro@127.0.0.1:5432/gomiro?sslmode=disable"),
		RedisAddr:        env("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword:    env("REDIS_PASSWORD", ""),
		UploadDir:        env("UPLOAD_DIR", "./data/uploads"),
		MaxUploadBytes:   envInt64("MAX_UPLOAD_BYTES", 5<<20),
		RoomIdle:         time.Duration(envInt("ROOM_IDLE_SEC", 45)) * time.Second,
		SnapshotInterval: time.Duration(envInt("SNAPSHOT_INTERVAL_SEC", 5)) * time.Second,
		EmptyRoomGrace:   time.Duration(envInt("EMPTY_ROOM_GRACE_SEC", 20)) * time.Second,
		LogLevel:         env("LOG_LEVEL", "info"),
		WSOrigin:         env("WS_ORIGIN", "*"),
		ProtoVersion:     int(envInt("PROTO_VERSION", 1)),
		MaxOpPerSec:      int(envInt("MAX_OP_PER_SEC", 200)),
		MaxMsgBytes:      int(envInt("MAX_MSG_BYTES", 256<<10)),
		PingInterval:     time.Duration(envInt("PING_INTERVAL_SEC", 30)) * time.Second,
		PongWait:         time.Duration(envInt("PONG_WAIT_SEC", 60)) * time.Second,
		WriteWait:        time.Duration(envInt("WRITE_WAIT_SEC", 10)) * time.Second,
		SendQueueSize:    int(envInt("SEND_QUEUE_SIZE", 256)),
		CursorTick:       time.Duration(envInt("CURSOR_TICK_MS", 33)) * time.Millisecond,
		BcryptCost:       int(envInt("BCRYPT_COST", 10)),
		MigrationsDir:    env("MIGRATIONS_DIR", "migrations"),
		MetricsNamespace: env("METRICS_NS", "gomiro"),
	}
	if c.NodeID == "" {
		return c, fmt.Errorf("NODE_ID is required")
	}
	if c.MaxMsgBytes < 1024 {
		return c, fmt.Errorf("MAX_MSG_BYTES too small")
	}
	if c.BcryptCost < 4 || c.BcryptCost > 14 {
		c.BcryptCost = 10
	}
	return c, nil
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int64) int64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return fallback
	}
	return n
}

func envInt64(key string, fallback int64) int64 {
	return envInt(key, fallback)
}

func hostnameFallback() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "node-local"
	}
	return h
}

func (c Config) Validate() error {
	if c.HTTPAddr == "" {
		return fmt.Errorf("HTTP_ADDR empty")
	}
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL empty")
	}
	if c.RedisAddr == "" {
		return fmt.Errorf("REDIS_ADDR empty")
	}
	if c.MaxUploadBytes < 1024 || c.MaxUploadBytes > 20<<20 {
		return fmt.Errorf("MAX_UPLOAD_BYTES out of range")
	}
	if c.MaxOpPerSec < 1 || c.MaxOpPerSec > 5000 {
		return fmt.Errorf("MAX_OP_PER_SEC out of range")
	}
	if c.SendQueueSize < 8 {
		return fmt.Errorf("SEND_QUEUE_SIZE too small")
	}
	if c.CursorTick < 10*time.Millisecond || c.CursorTick > time.Second {
		return fmt.Errorf("CURSOR_TICK_MS out of range")
	}
	if c.PingInterval < 5*time.Second {
		return fmt.Errorf("PING_INTERVAL_SEC too small")
	}
	if c.PongWait <= c.PingInterval {
		return fmt.Errorf("PONG_WAIT_SEC must exceed PING_INTERVAL_SEC")
	}
	if c.ProtoVersion != 1 {
		return fmt.Errorf("PROTO_VERSION must be 1")
	}
	return nil
}

func (c Config) Redacted() map[string]any {
	u := c.DatabaseURL
	if i := strings.Index(u, "@"); i > 0 {
		u = "postgres://***" + u[i:]
	}
	return map[string]any{
		"http": c.HTTPAddr, "node": c.NodeID, "db": u, "redis": c.RedisAddr,
		"upload": c.UploadDir, "maxUpload": c.MaxUploadBytes, "maxOp": c.MaxOpPerSec,
		"maxMsg": c.MaxMsgBytes, "queue": c.SendQueueSize, "cursorMs": c.CursorTick.Milliseconds(),
		"idleSec": c.RoomIdle.Seconds(), "snapSec": c.SnapshotInterval.Seconds(),
	}
}

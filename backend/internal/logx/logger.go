package logx

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"
)

type ctxKey string

const (
	keyBoard  ctxKey = "board_id"
	keyClient ctxKey = "client_id"
	keyNode   ctxKey = "node_id"
)

var (
	mu     sync.RWMutex
	logger *slog.Logger
)

func Init(level string, w io.Writer) {
	if w == nil {
		w = os.Stdout
	}
	var lv slog.Level
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		lv = slog.LevelDebug
	case "warn", "warning":
		lv = slog.LevelWarn
	case "error":
		lv = slog.LevelError
	default:
		lv = slog.LevelInfo
	}
	h := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: lv})
	mu.Lock()
	logger = slog.New(h)
	mu.Unlock()
}

func L() *slog.Logger {
	mu.RLock()
	defer mu.RUnlock()
	if logger == nil {
		return slog.Default()
	}
	return logger
}

func WithBoard(ctx context.Context, boardID string) context.Context {
	return context.WithValue(ctx, keyBoard, boardID)
}

func WithClient(ctx context.Context, clientID string) context.Context {
	return context.WithValue(ctx, keyClient, clientID)
}

func WithNode(ctx context.Context, nodeID string) context.Context {
	return context.WithValue(ctx, keyNode, nodeID)
}

func From(ctx context.Context) *slog.Logger {
	l := L()
	if ctx == nil {
		return l
	}
	if v, ok := ctx.Value(keyBoard).(string); ok && v != "" {
		l = l.With("board_id", v)
	}
	if v, ok := ctx.Value(keyClient).(string); ok && v != "" {
		l = l.With("client_id", v)
	}
	if v, ok := ctx.Value(keyNode).(string); ok && v != "" {
		l = l.With("node_id", v)
	}
	return l
}

func Info(msg string, args ...any)  { L().Info(msg, args...) }
func Warn(msg string, args ...any)  { L().Warn(msg, args...) }
func Error(msg string, args ...any) { L().Error(msg, args...) }
func Debug(msg string, args ...any) { L().Debug(msg, args...) }

func Board(boardID string) *slog.Logger {
	return L().With("board_id", boardID)
}

func Client(boardID, clientID string) *slog.Logger {
	return L().With("board_id", boardID, "client_id", clientID)
}

func DurationMs(start time.Time) int64 {
	return time.Since(start).Milliseconds()
}

func SafeErr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func Sampled(n uint64, every uint64) bool {
	if every == 0 {
		return true
	}
	return n%every == 0
}

func SanitizePath(p string) string {
	if p == "" {
		return "/"
	}
	if len(p) > 128 {
		return p[:128]
	}
	return p
}

func JoinAttrs(base []any, extra ...any) []any {
	out := make([]any, 0, len(base)+len(extra))
	out = append(out, base...)
	out = append(out, extra...)
	return out
}

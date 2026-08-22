package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"gomiro/internal/logx"
	"gomiro/internal/protocol"
)

const (
	ownerTTL   = 12 * time.Second
	ownerEvery = 4 * time.Second
)

type CursorFan struct {
	BoardID string                  `json:"boardId"`
	NodeID  string                  `json:"nodeId"`
	Samples []protocol.CursorSample `json:"samples"`
}

type InboxMsg struct {
	BoardID string          `json:"boardId"`
	NodeID  string          `json:"nodeId"`
	Client  string          `json:"clientId"`
	Kind    string          `json:"kind"` // op|selection
	Raw     json.RawMessage `json:"raw"`
}

type OutMsg struct {
	BoardID  string          `json:"boardId"`
	NodeID   string          `json:"nodeId"`
	Type     string          `json:"type"`
	Envelope json.RawMessage `json:"envelope,omitempty"`
	Binary   []byte          `json:"binary,omitempty"`
}

type Bus struct {
	rdb   *redis.Client
	node  string
	mu    sync.RWMutex
	ready bool
}

func New(addr, password, nodeID string) *Bus {
	rdb := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DialTimeout:  3 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     16,
	})
	return &Bus{rdb: rdb, node: nodeID}
}

// errRedisDisabled is returned when the bus has no underlying client.
// It is a stable sentinel so callers can distinguish "no client" from
// transport failures.
var errRedisDisabled = errors.New("redis disabled")

// ErrRedisDisabled returns the sentinel error used when the bus has no
// underlying client. Exported so callers can match it with errors.Is.
func ErrRedisDisabled() error { return errRedisDisabled }

// IsCanceled reports whether err (or any error in its tree) is a context
// cancellation. go-redis wraps cancellation as `context.DeadlineExceeded` /
// `context.Canceled` but also surfaces it through a *redis.Error that can
// carry the raw dial error. We unwrap both paths so a caller that cancels
// an upstream request is never told "Redis is down".
func IsCanceled(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	// go-redis sometimes returns the sentinel wrapped inside a net.OpError /
	// *redis.Error string. The cheapest robust check that does not couple us
	// to a private error type is a quick error-tree walk plus a string guard.
	if strings.Contains(err.Error(), "context deadline exceeded") ||
		strings.Contains(err.Error(), "context canceled") {
		return true
	}
	return false
}

// classify filters a raw go-redis error through the lens of the supplied
// context. When ctx is already done, dial/read timeouts triggered by the
// cancellation are rebranded from "Redis failure" to ctx.Err(), so callers
// can distinguish an active cancellation from a genuine outage.
func classify(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if ctx != nil && ctx.Err() != nil {
		// The context was cancelled/timed out. Any transport error observed
		// at this point is a side-effect of the cancellation, not a Redis
		// outage. Propagate the context error so callers can detect it via
		// errors.Is(err, context.Canceled|DeadlineExceeded).
		return ctx.Err()
	}
	if IsCanceled(err) {
		// Defensive: even without ctx.Err() set the error itself might be a
		// cancellation surfaced by the driver. Surface it as-is so errors.Is
		// against context.Canceled/DeadlineExceeded succeeds.
		return err
	}
	return err
}

func (b *Bus) Ping(ctx context.Context) error {
	if b == nil || b.rdb == nil {
		return errRedisDisabled
	}
	// Honour the caller's context. Previously we passed context.Background()
	// here, which meant a cancelled upstream request still kicked off a dial
	// against Redis; the resulting "dial tcp ..." error was then recorded as
	// a Redis outage by the health-check / retry loops.
	pingCtx := ctx
	if pingCtx == nil {
		pingCtx = context.Background()
	}
	return classify(pingCtx, b.rdb.Ping(pingCtx).Err())
}

func (b *Bus) Close() error {
	if b == nil || b.rdb == nil {
		return nil
	}
	return b.rdb.Close()
}

func (b *Bus) MarkReady(v bool) {
	b.mu.Lock()
	b.ready = v
	b.mu.Unlock()
}

func (b *Bus) Ready() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.ready
}

func ownerKey(boardID string) string { return "gomiro:owner:" + boardID }
func inboxCh(boardID string) string  { return "gomiro:inbox:" + boardID }
func outCh(boardID string) string    { return "gomiro:out:" + boardID }
func cursorCh(boardID string) string { return "gomiro:cursor:" + boardID }

func (b *Bus) TryOwn(ctx context.Context, boardID string) (bool, error) {
	ok, err := b.rdb.SetNX(ctx, ownerKey(boardID), b.node, ownerTTL).Result()
	if err != nil {
		return false, err
	}
	if ok {
		return true, nil
	}
	cur, err := b.rdb.Get(ctx, ownerKey(boardID)).Result()
	if err == redis.Nil {
		return b.TryOwn(ctx, boardID)
	}
	if err != nil {
		return false, err
	}
	if cur == b.node {
		_ = b.rdb.Expire(ctx, ownerKey(boardID), ownerTTL).Err()
		return true, nil
	}
	return false, nil
}

func (b *Bus) RenewOwn(ctx context.Context, boardID string) {
	script := redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("EXPIRE", KEYS[1], ARGV[2])
end
return 0`)
	_ = script.Run(ctx, b.rdb, []string{ownerKey(boardID)}, b.node, int(ownerTTL.Seconds())).Err()
}

func (b *Bus) Release(ctx context.Context, boardID string) {
	script := redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
end
return 0`)
	_ = script.Run(ctx, b.rdb, []string{ownerKey(boardID)}, b.node).Err()
}

func (b *Bus) PublishInbox(ctx context.Context, msg InboxMsg) error {
	raw, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return b.rdb.Publish(ctx, inboxCh(msg.BoardID), raw).Err()
}

func (b *Bus) PublishOut(ctx context.Context, msg OutMsg) error {
	raw, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return b.rdb.Publish(ctx, outCh(msg.BoardID), raw).Err()
}

func (b *Bus) PublishCursor(ctx context.Context, fan CursorFan) error {
	raw, err := json.Marshal(fan)
	if err != nil {
		return err
	}
	return b.rdb.Publish(ctx, cursorCh(fan.BoardID), raw).Err()
}

type Handlers struct {
	OnInbox  func(InboxMsg)
	OnOut    func(OutMsg)
	OnCursor func(CursorFan)
}

func (b *Bus) SubscribeBoard(ctx context.Context, boardID string, h Handlers) (func(), error) {
	ps := b.rdb.Subscribe(ctx, inboxCh(boardID), outCh(boardID), cursorCh(boardID))
	ch := ps.Channel()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case m, ok := <-ch:
				if !ok {
					return
				}
				b.dispatch(m, h)
			}
		}
	}()
	return func() { _ = ps.Close() }, nil
}

func (b *Bus) dispatch(m *redis.Message, h Handlers) {
	if m == nil {
		return
	}
	switch {
	case strings.HasPrefix(m.Channel, "gomiro:inbox:"):
		var msg InboxMsg
		if json.Unmarshal([]byte(m.Payload), &msg) == nil && h.OnInbox != nil {
			h.OnInbox(msg)
		}
	case strings.HasPrefix(m.Channel, "gomiro:out:"):
		var msg OutMsg
		if json.Unmarshal([]byte(m.Payload), &msg) == nil {
			if msg.NodeID == b.node {
				return
			}
			if h.OnOut != nil {
				h.OnOut(msg)
			}
		}
	case strings.HasPrefix(m.Channel, "gomiro:cursor:"):
		var fan CursorFan
		if json.Unmarshal([]byte(m.Payload), &fan) == nil {
			if fan.NodeID == b.node {
				return
			}
			if h.OnCursor != nil {
				h.OnCursor(fan)
			}
		}
	default:
		logx.L().Debug("unknown redis channel", "ch", m.Channel)
	}
}

func OwnerRenewPeriod() time.Duration { return ownerEvery }

func InboxChannel(boardID string) string  { return inboxCh(boardID) }
func OutChannel(boardID string) string    { return outCh(boardID) }
func CursorChannel(boardID string) string { return cursorCh(boardID) }
func OwnerKey(boardID string) string      { return ownerKey(boardID) }

func (b *Bus) CurrentOwner(ctx context.Context, boardID string) (string, bool) {
	v, err := b.rdb.Get(ctx, ownerKey(boardID)).Result()
	if err != nil || v == "" {
		return "", false
	}
	return v, true
}

func (b *Bus) WaitReady(ctx context.Context, attempts int) error {
	if attempts <= 0 {
		attempts = 10
	}
	var last error
	for i := 0; i < attempts; i++ {
		last = b.Ping(ctx)
		if last == nil {
			b.MarkReady(true)
			return nil
		}
		// If the caller gave up, surface the cancellation immediately rather
		// than retrying and ultimately reporting a dial error as "Redis down".
		if IsCanceled(last) || (ctx != nil && ctx.Err() != nil) {
			if ctx != nil && ctx.Err() != nil {
				return ctx.Err()
			}
			return last
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	// All attempts exhausted. If the final error was just a cancellation, do
	// not mislabel it as a Redis outage.
	if IsCanceled(last) {
		if ctx != nil && ctx.Err() != nil {
			return ctx.Err()
		}
		return last
	}
	return last
}

func (b *Bus) Node() string { return b.node }

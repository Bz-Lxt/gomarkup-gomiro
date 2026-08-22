package cluster

import (
	"context"
	"encoding/json"
	"fmt"
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
	BoardID string            `json:"boardId"`
	NodeID  string            `json:"nodeId"`
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
	rdb    *redis.Client
	node   string
	mu     sync.RWMutex
	ready  bool
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

func (b *Bus) Ping(ctx context.Context) error {
	if b == nil || b.rdb == nil {
		return fmt.Errorf("redis disabled")
	}
	err := b.rdb.Ping(ctx).Err()
	if err == nil {
		b.MarkReady(true)
	} else {
		b.MarkReady(false)
	}
	return err
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
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	return last
}

func (b *Bus) Node() string { return b.node }

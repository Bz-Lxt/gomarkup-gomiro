package ws

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"gomiro/internal/config"
	"gomiro/internal/model"
	"gomiro/internal/protocol"
)

type outbound struct {
	text   []byte
	bin    []byte
	cursor bool
}

type Client struct {
	ID       string
	UserIdx  uint32
	Nickname string
	Color    string
	BoardID  string
	Lamport  uint64
	LastSeq  uint64

	conn *websocket.Conn
	cfg  config.Config
	send chan outbound

	mu     sync.Mutex
	closed bool
	joined atomic.Bool

	cursorX float32
	cursorY float32
	sel     []string

	opWindow []time.Time
	dropCur  int
}

func newClient(conn *websocket.Conn, cfg config.Config) *Client {
	return &Client{
		ID:   model.NewID("c"),
		conn: conn,
		cfg:  cfg,
		send: make(chan outbound, cfg.SendQueueSize),
	}
}

func (c *Client) EnqueueText(b []byte) bool {
	return c.enqueue(outbound{text: b}, false)
}

func (c *Client) EnqueueBin(b []byte, cursor bool) bool {
	return c.enqueue(outbound{bin: b, cursor: cursor}, cursor)
}

func (c *Client) enqueue(m outbound, droppable bool) bool {
	if c == nil || c.send == nil {
		return true
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return false
	}
	c.mu.Unlock()
	select {
	case c.send <- m:
		return true
	default:
		if droppable {
			return false
		}
		// last-ditch: drop one cursor-class frame then retry once
		select {
		case old := <-c.send:
			if !old.cursor {
				// put it back if we can; otherwise we are a slow client
				select {
				case c.send <- old:
				default:
					c.Close()
					return false
				}
				c.Close()
				return false
			}
			select {
			case c.send <- m:
				return true
			default:
				c.Close()
				return false
			}
		default:
			c.Close()
			return false
		}
	}
}

func (c *Client) Close() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	c.mu.Unlock()
	_ = c.conn.Close()
}

func (c *Client) Closed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

func (c *Client) writePump(stop <-chan struct{}) {
	ping := time.NewTicker(c.cfg.PingInterval)
	defer ping.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ping.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(c.cfg.WriteWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.Close()
				return
			}
		case m, ok := <-c.send:
			if !ok {
				return
			}
			_ = c.conn.SetWriteDeadline(time.Now().Add(c.cfg.WriteWait))
			var err error
			if m.bin != nil {
				err = c.conn.WriteMessage(websocket.BinaryMessage, m.bin)
			} else {
				err = c.conn.WriteMessage(websocket.TextMessage, m.text)
			}
			if err != nil {
				c.Close()
				return
			}
		}
	}
}

func (c *Client) noteOp(now time.Time, max int) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	cut := now.Add(-time.Second)
	kept := c.opWindow[:0]
	for _, t := range c.opWindow {
		if t.After(cut) {
			kept = append(kept, t)
		}
	}
	c.opWindow = kept
	if len(c.opWindow) >= max {
		return false
	}
	c.opWindow = append(c.opWindow, now)
	return true
}

func (c *Client) SetCursor(x, y float32) {
	c.mu.Lock()
	c.cursorX, c.cursorY = x, y
	c.mu.Unlock()
}

func (c *Client) Cursor() (float32, float32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cursorX, c.cursorY
}

func (c *Client) SetSel(ids []string) {
	c.mu.Lock()
	c.sel = append([]string(nil), ids...)
	c.mu.Unlock()
}

func (c *Client) Member() model.Member {
	x, y := c.Cursor()
	return model.Member{
		ID:       c.ID,
		Nickname: c.Nickname,
		Color:    c.Color,
		UserIdx:  c.UserIdx,
		CursorX:  float64(x),
		CursorY:  float64(y),
		Online:   true,
	}
}

func encodeErr(code, msg string) []byte {
	b, _ := protocol.Encode(protocol.TypeError, "", protocol.ErrorPayload{Code: code, Message: msg})
	return b
}

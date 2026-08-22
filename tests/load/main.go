// 1000-connection soak against a live GoMiro node. Not part of the 25–35 backend budget.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	n := flag.Int("n", 1000, "connections")
	base := flag.String("base", "http://127.0.0.1:18432", "api base")
	hold := flag.Duration("hold", 60*time.Second, "hold time")
	flag.Parse()
	board := mustCreate(*base)
	wsURL := strings.Replace(strings.Replace(*base, "http://", "ws://", 1), "https://", "wss://", 1) + "/ws"
	var alive, fail atomic.Int64
	var wg sync.WaitGroup
	stop := time.After(*hold)
	for i := 0; i < *n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
			if err != nil {
				fail.Add(1)
				return
			}
			defer c.Close()
			join, _ := json.Marshal(map[string]any{
				"v": 1, "type": "join",
				"payload": map[string]any{
					"boardId": board, "nickname": fmt.Sprintf("p%d", i),
					"color": "#2f5d56", "lastSeq": 0, "protoVersion": 1,
				},
			})
			if err := c.WriteMessage(websocket.TextMessage, join); err != nil {
				fail.Add(1)
				return
			}
			alive.Add(1)
			c.SetReadDeadline(time.Now().Add(*hold + 5*time.Second))
			for {
				select {
				case <-stop:
					return
				default:
					if _, _, err := c.ReadMessage(); err != nil {
						return
					}
				}
			}
		}(i)
		if i%50 == 0 {
			time.Sleep(20 * time.Millisecond)
		}
	}
	wg.Wait()
	fmt.Printf("alive=%d fail=%d\n", alive.Load(), fail.Load())
	if alive.Load() < int64(*n)*90/100 {
		os.Exit(1)
	}
}

func mustCreate(base string) string {
	resp, err := http.Post(base+"/api/v1/boards", "application/json", strings.NewReader(`{"title":"load"}`))
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(raw, &out)
	if out.ID == "" {
		panic("create board failed: " + string(raw))
	}
	return out.ID
}

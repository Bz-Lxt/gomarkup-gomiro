package ws_test

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	"gomiro/internal/ws"
)

func TestClientCursorPairRemainsAtomic(t *testing.T) {
	var client ws.Client
	client.SetCursor(1, 1)

	var start sync.WaitGroup
	start.Add(1)
	var active atomic.Int32
	var torn atomic.Bool

	writers := runtime.GOMAXPROCS(0) * 2
	if writers < 4 {
		writers = 4
	}
	if writers > 8 {
		writers = 8
	}
	active.Store(int32(writers))
	var wg sync.WaitGroup
	for i := 1; i <= writers; i++ {
		value := float32(i)
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer active.Add(-1)
			start.Wait()
			for j := 0; j < 10_000; j++ {
				client.SetCursor(value, value)
			}
		}()
	}

	start.Done()
	for active.Load() > 0 {
		x, y := client.Cursor()
		if x != y {
			torn.Store(true)
			break
		}
	}
	wg.Wait()

	if torn.Load() {
		t.Fatal("Cursor returned coordinates that were never submitted as a pair")
	}
}

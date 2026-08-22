package protocol_test

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"

	"gomiro/internal/protocol"
)

func TestEncodeConcurrentPayloadIsolation(t *testing.T) {
	type payload struct {
		Token string `json:"token"`
		Body  string `json:"body"`
	}

	workers := runtime.GOMAXPROCS(0) * 2
	if workers < 8 {
		workers = 8
	}
	start := make(chan struct{})
	failures := make(chan string, 1)
	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		worker := worker
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for round := 0; round < 64; round++ {
				token := fmt.Sprintf("worker-%d-round-%d", worker, round)
				want := payload{Token: token, Body: strings.Repeat(token+"/", 128)}
				raw, err := protocol.Encode("room_event", token, want)
				if err != nil {
					reportEncodeFailure(failures, fmt.Sprintf("encode %s: %v", token, err))
					return
				}
				env, err := protocol.Decode(raw)
				if err != nil {
					reportEncodeFailure(failures, fmt.Sprintf("decode %s: %v; raw=%q", token, err, raw))
					return
				}
				got, err := protocol.DecodePayload[payload](env.Payload)
				if err != nil {
					reportEncodeFailure(failures, fmt.Sprintf("decode payload %s: %v; raw=%q", token, err, raw))
					return
				}
				if env.ID != token || env.Type != "room_event" || got != want {
					reportEncodeFailure(failures, fmt.Sprintf("message %s crossed streams: envelope id=%q type=%q payload token=%q body-bytes=%d", token, env.ID, env.Type, got.Token, len(got.Body)))
					return
				}
			}
		}()
	}
	close(start)
	wg.Wait()
	select {
	case failure := <-failures:
		t.Fatal(failure)
	default:
	}
}

func reportEncodeFailure(dst chan<- string, failure string) {
	select {
	case dst <- failure:
	default:
	}
}

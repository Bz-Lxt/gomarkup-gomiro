package cluster_test

import (
	"context"
	"errors"
	"testing"

	"gomiro/internal/cluster"
)

func TestBusPingPreservesCallerCancellation(t *testing.T) {
	bus := cluster.New("127.0.0.1:0", "", "test-node")
	defer bus.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := bus.Ping(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Ping error = %v, want context.Canceled in the error chain", err)
	}
}

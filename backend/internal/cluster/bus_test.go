package cluster

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"
)

// dialBlackhole returns an address where no server listens but which TCP
// makes a connection refused (not "no route"), forcing a dial failure.
// Using a routable-but-closed port keeps the failure path deterministic
// without flaky timing on a real unreachable IP.
func dialBlackhole(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := l.Addr().String()
	_ = l.Close() // immediately close so the port is free → dial fails
	return addr
}

// TestPing_PreCancelledContextPropagates verifies the core regression: when
// the upstream caller has already cancelled, Ping must NOT issue a fresh
// dial attempt and must surface context.Canceled (not a network error).
func TestPing_PreCancelledContextPropagates(t *testing.T) {
	addr := dialBlackhole(t)
	b := New(addr, "", "node-1")
	t.Cleanup(func() { _ = b.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel BEFORE calling Ping

	err := b.Ping(ctx)
	if err == nil {
		t.Fatalf("Ping: expected error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Ping: expected context.Canceled, got %v", err)
	}
	// The error must NOT look like a dial failure — that's the whole point.
	if strings.Contains(err.Error(), "dial tcp") {
		t.Fatalf("Ping: cancellation surfaced as dial error: %v", err)
	}
}

// TestPing_ContextCancelledDuringDial mirrors the real-world race: the
// caller cancels while the dial is pending. The driver returns a wrapped
// dial error; Ping must still classify it as cancellation so the health
// check / retry loops don't record a Redis outage.
func TestPing_ContextCancelledDuringDial(t *testing.T) {
	// Use a routable but black-holed address: connections hang until the
	// dialer's own timeout. We cancel well before that, so the driver
	// observes ctx.Err() and wraps it into a dial error.
	b := New("10.255.255.1:6379", "", "node-1")
	t.Cleanup(func() { _ = b.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	err := b.Ping(ctx)
	if err == nil {
		t.Skip("dial succeeded unexpectedly; cannot assert cancellation")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

// TestPing_RealTimeoutClassifiedAsDeadline confirms that a plain deadline
// (not an upstream cancel) still surfaces as context.DeadlineExceeded
// rather than a raw dial error, so /readyz treats it as "unknown".
func TestPing_RealTimeoutClassifiedAsDeadline(t *testing.T) {
	// A routable-but-unroutable IP: dial hangs until the context deadline.
	b := New("10.255.255.1:6379", "", "node-1")
	t.Cleanup(func() { _ = b.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()

	err := b.Ping(ctx)
	if err == nil {
		t.Skip("dial succeeded unexpectedly; cannot assert timeout")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
}

// TestPing_GenuineDialErrorUntouched ensures we don't over-fit: when there
// is no cancellation, a real connection-refused must still be returned as
// a network error so the health check can correctly flag Redis as down.
func TestPing_GenuineDialErrorUntouched(t *testing.T) {
	addr := dialBlackhole(t)
	b := New(addr, "", "node-1")
	t.Cleanup(func() { _ = b.Close() })

	// Short timeout so the test is fast; no cancellation here.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := b.Ping(ctx)
	if err == nil {
		t.Fatalf("expected dial error, got nil")
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("genuine dial error misclassified as cancellation: %v", err)
	}
}

// TestWaitReady_CancelledReturnsCancellation locks in the retry loop: a
// cancelled context must short-circuit the loop and return ctx.Err(),
// not the last dial error.
func TestWaitReady_CancelledReturnsCancellation(t *testing.T) {
	addr := dialBlackhole(t)
	b := New(addr, "", "node-1")
	t.Cleanup(func() { _ = b.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancelled

	err := b.WaitReady(ctx, 5)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("WaitReady: expected context.Canceled, got %v", err)
	}
	if strings.Contains(err.Error(), "dial tcp") {
		t.Fatalf("WaitReady: cancellation surfaced as dial error: %v", err)
	}
}

// TestErrRedisDisabled_Stable ensures the "no client" path is detectable
// with errors.Is (so /readyz can distinguish "disabled" from "down").
func TestErrRedisDisabled_Stable(t *testing.T) {
	var b *Bus
	err := b.Ping(context.Background())
	if !errors.Is(err, errRedisDisabled) {
		t.Fatalf("expected errRedisDisabled, got %v", err)
	}
	if !errors.Is(err, ErrRedisDisabled()) {
		t.Fatalf("ErrRedisDisabled() must match Ping error")
	}
}

// TestIsCanceled covers the error-tree detection used by /readyz and main.
func TestIsCanceled(t *testing.T) {
	if IsCanceled(nil) {
		t.Fatal("nil is not cancelled")
	}
	if !IsCanceled(context.Canceled) {
		t.Fatal("context.Canceled not detected")
	}
	if !IsCanceled(context.DeadlineExceeded) {
		t.Fatal("context.DeadlineExceeded not detected")
	}
	if !IsCanceled(errors.New("dial tcp: context deadline exceeded")) {
		t.Fatal("wrapped deadline string not detected")
	}
	if IsCanceled(errors.New("connection refused")) {
		t.Fatal("refused misclassified as cancellation")
	}
}

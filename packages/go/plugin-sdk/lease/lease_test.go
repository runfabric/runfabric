package lease

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestDefaultInterval(t *testing.T) {
	if got := DefaultInterval(30 * time.Second); got != 10*time.Second {
		t.Fatalf("DefaultInterval(30s) = %v, want 10s", got)
	}
	if got := DefaultInterval(100 * time.Millisecond); got != time.Second {
		t.Fatalf("DefaultInterval(100ms) = %v, want clamp to 1s", got)
	}
}

func TestStartHeartbeatRenewsAndStopsOnError(t *testing.T) {
	var renews atomic.Int64
	boom := errors.New("lease lost")

	errCh := StartHeartbeat(context.Background(), func(time.Duration) error {
		if renews.Add(1) >= 3 {
			return boom
		}
		return nil
	}, time.Second, 5*time.Millisecond)

	select {
	case err := <-errCh:
		if !errors.Is(err, boom) {
			t.Fatalf("heartbeat error = %v, want %v", err, boom)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("heartbeat did not report renew error")
	}
	if _, ok := <-errCh; ok {
		t.Fatal("channel should close after the error")
	}
	if renews.Load() != 3 {
		t.Fatalf("renews = %d, want 3", renews.Load())
	}
}

func TestStartHeartbeatCancelClosesCleanly(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	errCh := StartHeartbeat(ctx, func(time.Duration) error { return nil }, time.Second, 5*time.Millisecond)
	cancel()
	select {
	case err, ok := <-errCh:
		if ok {
			t.Fatalf("expected clean close, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("heartbeat did not exit on cancel")
	}
}

func TestStartHeartbeatNilRenewClosesImmediately(t *testing.T) {
	errCh := StartHeartbeat(context.Background(), nil, time.Second, time.Millisecond)
	select {
	case _, ok := <-errCh:
		if ok {
			t.Fatal("nil renew should close without sending")
		}
	case <-time.After(time.Second):
		t.Fatal("nil renew did not close the channel")
	}
}

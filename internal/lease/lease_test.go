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

func TestStartHeartbeatRenewsUntilCancel(t *testing.T) {
	var renews atomic.Int64
	ctx, cancel := context.WithCancel(context.Background())

	errCh := StartHeartbeat(ctx, func(time.Duration) error {
		renews.Add(1)
		return nil
	}, time.Second, 5*time.Millisecond)

	deadline := time.After(2 * time.Second)
	for renews.Load() < 3 {
		select {
		case <-deadline:
			t.Fatalf("only %d renewals before deadline", renews.Load())
		case <-time.After(5 * time.Millisecond):
		}
	}
	cancel()

	if err, ok := <-errCh; ok {
		t.Fatalf("expected clean close after cancel, got %v", err)
	}
}

func TestStartHeartbeatStopsOnRenewError(t *testing.T) {
	boom := errors.New("lease lost")
	errCh := StartHeartbeat(context.Background(), func(time.Duration) error {
		return boom
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

func TestManagedReleaseStopsHeartbeatBeforeReleasing(t *testing.T) {
	var order []string
	var renewing atomic.Bool
	renewing.Store(true)

	m := Manage(context.Background(), func(time.Duration) error {
		if !renewing.Load() {
			t.Error("renew ran after Release returned")
		}
		return nil
	}, time.Second, 5*time.Millisecond, func() error {
		order = append(order, "release")
		return nil
	})

	time.Sleep(20 * time.Millisecond)
	if err := m.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
	renewing.Store(false)
	order = append(order, "returned")

	if len(order) != 2 || order[0] != "release" {
		t.Fatalf("order = %v, want release before return", order)
	}
}

func TestManagedReleaseIsIdempotent(t *testing.T) {
	var releases atomic.Int64
	boom := errors.New("release failed")
	m := Manage(context.Background(), nil, time.Second, time.Millisecond, func() error {
		releases.Add(1)
		return boom
	})

	if err := m.Release(); !errors.Is(err, boom) {
		t.Fatalf("first Release = %v, want %v", err, boom)
	}
	if err := m.Release(); !errors.Is(err, boom) {
		t.Fatalf("repeat Release = %v, want first result %v", err, boom)
	}
	if releases.Load() != 1 {
		t.Fatalf("release ran %d times, want once", releases.Load())
	}

	var nilManaged *Managed
	if err := nilManaged.Release(); err != nil {
		t.Fatalf("nil Release = %v, want nil", err)
	}
}

func TestManagedHeartbeatErrSurfacesLeaseLoss(t *testing.T) {
	boom := errors.New("stolen")
	m := Manage(context.Background(), func(time.Duration) error {
		return boom
	}, time.Second, 5*time.Millisecond, nil)

	select {
	case err := <-m.HeartbeatErr():
		if !errors.Is(err, boom) {
			t.Fatalf("HeartbeatErr = %v, want %v", err, boom)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("lease loss not surfaced")
	}
	if err := m.Release(); err != nil {
		t.Fatalf("Release after loss: %v", err)
	}
}

// Package lease is the engine's single lease-keepalive primitive. Every
// lock/lease implementation on the platform side (deploy-state locks, workflow
// run locks) drives its renewal through StartHeartbeat or Manage instead of
// hand-rolling a ticker loop, so TTL and renewal semantics stay uniform:
//
//   - a lease is held for leaseFor and renewed every interval (DefaultInterval
//     recommends leaseFor/3, clamped to >= 1s);
//   - a crashed holder stops renewing, so its lease goes stale after leaseFor
//     and can be stolen by the backend's stale/steal rule;
//   - the first renewal error stops the heartbeat and is reported once on the
//     returned channel — the renew callback decides what is fatal (return the
//     error) versus transient (swallow it and return nil).
//
// Plugin authors (root extensions/... packages) cannot import this package;
// the plugin-sdk ships the same helper for that side of the boundary.
package lease

import (
	"context"
	"sync"
	"time"
)

// RenewFunc extends the holder's lease by leaseFor. Returning a non-nil error
// stops the heartbeat and reports the error; implementations should return nil
// for failures worth retrying on the next tick.
type RenewFunc func(leaseFor time.Duration) error

// DefaultInterval returns the renewal cadence for a lease TTL: ttl/3 so a
// holder gets multiple renewal attempts per lease window, clamped to >= 1s so
// short TTLs do not busy-loop the backend.
func DefaultInterval(ttl time.Duration) time.Duration {
	interval := ttl / 3
	if interval < time.Second {
		interval = time.Second
	}
	return interval
}

// StartHeartbeat renews the lease every interval until ctx is done or renew
// returns an error. The returned channel receives at most one error (the renew
// failure that stopped the heartbeat) and is closed when the heartbeat exits,
// so callers can both watch for lease loss and wait for goroutine shutdown.
// A nil renew exits immediately with a closed channel.
func StartHeartbeat(ctx context.Context, renew RenewFunc, leaseFor, interval time.Duration) <-chan error {
	errCh := make(chan error, 1)

	go func() {
		defer close(errCh)

		if renew == nil {
			return
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := renew(leaseFor); err != nil {
					errCh <- err
					return
				}
			}
		}
	}()

	return errCh
}

// Managed couples a held lease with its heartbeat and release: Manage starts
// the heartbeat, Release stops it, waits for the goroutine to exit, and then
// releases the underlying lease exactly once.
type Managed struct {
	cancel  context.CancelFunc
	errCh   <-chan error
	release func() error

	once       sync.Once
	releaseErr error
}

// Manage starts a heartbeat for a lease that was just acquired. ctx bounds the
// heartbeat's lifetime in addition to Release; release may be nil when the
// backend has nothing to clean up.
func Manage(ctx context.Context, renew RenewFunc, leaseFor, interval time.Duration, release func() error) *Managed {
	hbCtx, cancel := context.WithCancel(ctx)
	return &Managed{
		cancel:  cancel,
		errCh:   StartHeartbeat(hbCtx, renew, leaseFor, interval),
		release: release,
	}
}

// Release stops the heartbeat, waits for it to exit, and releases the lease.
// Safe to call more than once and on a nil receiver; repeat calls return the
// first release result.
func (m *Managed) Release() error {
	if m == nil {
		return nil
	}
	m.once.Do(func() {
		m.cancel()
		// Drain until closed: guarantees the heartbeat goroutine has exited
		// before the lease is released, so a late renewal cannot resurrect a
		// lock another holder is about to take.
		for range m.errCh {
		}
		if m.release != nil {
			m.releaseErr = m.release()
		}
	})
	return m.releaseErr
}

// HeartbeatErr exposes the heartbeat's error channel: it yields the renewal
// error that stopped the heartbeat (at most one) and closes on exit. Callers
// use it to abort work when the lease is lost mid-operation.
func (m *Managed) HeartbeatErr() <-chan error {
	if m == nil {
		return nil
	}
	return m.errCh
}

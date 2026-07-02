// Package lease provides the lease-keepalive helper for plugin authors. State
// backend plugins that hold TTL-based locks renew them through StartHeartbeat
// instead of hand-rolling a ticker loop, so lease semantics stay uniform with
// the engine:
//
//   - a lease is held for leaseFor and renewed every interval (DefaultInterval
//     recommends leaseFor/3, clamped to >= 1s);
//   - a crashed holder stops renewing, so its lease goes stale after leaseFor
//     and can be stolen by the backend's stale/steal rule;
//   - the first renewal error stops the heartbeat and is reported once on the
//     returned channel — the renew callback decides what is fatal (return the
//     error) versus transient (swallow it and return nil).
//
// This mirrors the engine-side primitive in internal/lease, which plugins
// cannot import across the extension boundary.
package lease

import (
	"context"
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

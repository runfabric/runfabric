package controlplane

import (
	"context"
	"time"

	"github.com/runfabric/runfabric/platform/core/state/locking"
)

func StartHeartbeat(ctx context.Context, handle *locking.Handle, leaseFor, interval time.Duration) <-chan error {
	errCh := make(chan error, 1)

	go func() {
		defer close(errCh)

		if handle == nil {
			return
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := handle.Renew(leaseFor); err != nil {
					errCh <- err
					return
				}
			}
		}
	}()

	return errCh
}

func FailIfLeaseLost(ctx context.Context, lock *ManagedLock, service, stage string) error {
	if lock == nil {
		return nil
	}

	select {
	case <-ctx.Done():
		return nil
	case err, ok := <-lock.HeartbeatErr():
		if !ok {
			return nil
		}
		if err != nil {
			return &LeaseLostError{
				Service: service,
				Stage:   stage,
				Message: err.Error(),
			}
		}
		return nil
	default:
		return nil
	}
}

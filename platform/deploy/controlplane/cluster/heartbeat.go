package cluster

import (
	"context"
)

// Lease renewal runs through the shared primitive in internal/lease (see
// LockManager.Acquire); this file keeps the controlplane-specific policy for
// reacting to a lost lease.

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

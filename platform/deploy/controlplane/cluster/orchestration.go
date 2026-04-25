package cluster

import (
	"context"
	"fmt"
)

func WatchHeartbeat(ctx context.Context, lock *ManagedLock) error {
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
			return fmt.Errorf("lock heartbeat lost: %w", err)
		}
		return nil
	default:
		return nil
	}
}

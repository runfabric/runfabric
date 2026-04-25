package cluster

import (
	"context"
	"fmt"
)

type LeaseLostError struct {
	Service string
	Stage   string
	Message string
}

func (e *LeaseLostError) Error() string {
	return fmt.Sprintf("lease lost for service=%s stage=%s: %s", e.Service, e.Stage, e.Message)
}

func AbortIfLeaseLost(
	ctx context.Context,
	lock *ManagedLock,
	service string,
	stage string,
) error {
	select {
	case err, ok := <-lock.HeartbeatErr():
		if !ok {
			return nil
		}
		if err != nil {
			return &LeaseLostError{
				Service: service,
				Stage:   stage,
				Message: "heartbeat renewal failed",
			}
		}
	default:
	}
	return nil
}

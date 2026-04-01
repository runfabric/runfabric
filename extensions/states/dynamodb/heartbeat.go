package dynamodb

import (
	"context"
	"time"

	statetypes "github.com/runfabric/runfabric/extensions/types"
)

func StartHeartbeat(ctx context.Context, handle *statetypes.Handle, leaseFor time.Duration, interval time.Duration) <-chan error {
	errCh := make(chan error, 1)

	go func() {
		defer close(errCh)

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

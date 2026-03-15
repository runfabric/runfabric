package dynamodb

import (
	"context"
	"time"

	"github.com/runfabric/runfabric/internal/locking"
)

func StartHeartbeat(ctx context.Context, handle *locking.Handle, leaseFor time.Duration, interval time.Duration) <-chan error {
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

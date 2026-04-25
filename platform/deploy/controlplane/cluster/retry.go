package cluster

import (
	"context"
	"time"
)

func Retry(ctx context.Context, attempts int, fn func() error) error {
	var lastErr error

	for i := 1; i <= attempts; i++ {
		if err := fn(); err == nil {
			return nil
		} else {
			lastErr = err
		}

		if i < attempts {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(Backoff(i)):
			}
		}
	}

	return lastErr
}

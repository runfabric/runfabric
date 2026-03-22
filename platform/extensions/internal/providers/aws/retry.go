package aws

import (
	"context"
	"fmt"
	"time"
)

func retry(ctx context.Context, attempts int, delay time.Duration, fn func() error) error {
	var lastErr error

	for i := 0; i < attempts; i++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		err := fn()
		if err == nil {
			return nil
		}
		lastErr = err

		if i < attempts-1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("retry failed with unknown error")
	}
	return lastErr
}

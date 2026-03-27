package gcp

import (
	"context"
	"time"
)

func retryWithBackoff(ctx context.Context, maxAttempts int, initialBackoff time.Duration, fn func() error) error {
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	backoff := initialBackoff
	if backoff <= 0 {
		backoff = 100 * time.Millisecond
	}
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := fn(); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if attempt == maxAttempts {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		backoff *= 2
	}
	return lastErr
}

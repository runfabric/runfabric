package apiutil

import (
	"context"
	"time"
)

// RetryWithBackoff runs fn up to maxAttempts times. On error it waits backoff before retrying.
// Backoff doubles each time (e.g. 200ms, 400ms, 800ms). Context cancellation stops retries.
// Use for noisy provider APIs (e.g. GCP Logging, Azure Log Analytics) to avoid failing on transient errors.
func RetryWithBackoff(ctx context.Context, maxAttempts int, initialBackoff time.Duration, fn func() error) error {
	var lastErr error
	backoff := initialBackoff
	if backoff <= 0 {
		backoff = 200 * time.Millisecond
	}
	for attempt := 0; attempt < maxAttempts; attempt++ {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}
		if attempt == maxAttempts-1 {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
			backoff *= 2
			if backoff > 5*time.Second {
				backoff = 5 * time.Second
			}
		}
	}
	return lastErr
}

package controlplane

import "time"

func Backoff(attempt int) time.Duration {
	if attempt <= 1 {
		return 200 * time.Millisecond
	}
	if attempt == 2 {
		return 500 * time.Millisecond
	}
	if attempt == 3 {
		return 1 * time.Second
	}
	return 2 * time.Second
}

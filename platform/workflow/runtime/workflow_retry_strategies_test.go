package runtime

import (
	"errors"
	"testing"
)

// TestRetryStrategies_CapAtMaxAttempts guards the off-by-one where ShouldRetry
// allowed one execution beyond MaxAttempts. ShouldRetry(attempt) is called after
// the attempt-th execution, so it must return false once attempt == MaxAttempts.
func TestRetryStrategies_CapAtMaxAttempts(t *testing.T) {
	err := errors.New("ThrottlingException RESOURCE_EXHAUSTED 429 throttl")

	cases := []struct {
		name string
		s    RetryStrategy
		max  int
	}{
		{"default", DefaultRetryStrategy{MaxAttempts: 3}, 3},
		{"aws", AWSRetryStrategy{MaxAttempts: 2}, 2},
		{"gcp", GCPRetryStrategy{MaxAttempts: 4}, 4},
		{"azure", AzureRetryStrategy{MaxAttempts: 2}, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for attempt := 1; attempt < c.max; attempt++ {
				if !c.s.ShouldRetry(attempt, err) {
					t.Errorf("attempt %d/%d: expected retry", attempt, c.max)
				}
			}
			if c.s.ShouldRetry(c.max, err) {
				t.Errorf("attempt %d == MaxAttempts: must not retry (would exceed MaxAttempts executions)", c.max)
			}
		})
	}
}

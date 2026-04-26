package runtime

import (
	"strings"
	"time"
)

// RetryStrategy decides whether to retry a failed MCP call and how long to wait.
// Implementations handle provider-specific transient error patterns.
type RetryStrategy interface {
	// ShouldRetry returns true when the error is retriable for the given attempt (1-based).
	ShouldRetry(attempt int, err error) bool
	// Backoff returns the duration to wait before the next attempt (1-based).
	Backoff(attempt int) time.Duration
}

// DefaultRetryStrategy applies a simple linear backoff with no error discrimination.
type DefaultRetryStrategy struct {
	MaxAttempts int
	BaseBackoff time.Duration
}

func (s DefaultRetryStrategy) ShouldRetry(attempt int, err error) bool {
	max := s.MaxAttempts
	if max <= 0 {
		max = 3
	}
	return err != nil && attempt <= max
}

func (s DefaultRetryStrategy) Backoff(attempt int) time.Duration {
	base := s.BaseBackoff
	if base <= 0 {
		base = 100 * time.Millisecond
	}
	return base * time.Duration(attempt)
}

// AWSRetryStrategy handles AWS Bedrock / SDK transient errors.
// Retries ThrottlingException and ServiceUnavailableException with exponential backoff + cap.
type AWSRetryStrategy struct {
	MaxAttempts int
}

func (s AWSRetryStrategy) ShouldRetry(attempt int, err error) bool {
	max := s.MaxAttempts
	if max <= 0 {
		max = 5
	}
	if attempt > max || err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "ThrottlingException") ||
		strings.Contains(msg, "ServiceUnavailableException") ||
		strings.Contains(msg, "TooManyRequestsException") ||
		strings.Contains(msg, "throttl")
}

func (s AWSRetryStrategy) Backoff(attempt int) time.Duration {
	// Exponential: 100ms * 2^(attempt-1), capped at 10s.
	d := 100 * time.Millisecond * (1 << uint(attempt-1))
	if d > 10*time.Second {
		d = 10 * time.Second
	}
	return d
}

// GCPRetryStrategy handles GCP Vertex AI / Cloud API transient errors.
// RESOURCE_EXHAUSTED quota errors require longer backoff than typical rate limits.
type GCPRetryStrategy struct {
	MaxAttempts int
}

func (s GCPRetryStrategy) ShouldRetry(attempt int, err error) bool {
	max := s.MaxAttempts
	if max <= 0 {
		max = 4
	}
	if attempt > max || err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "RESOURCE_EXHAUSTED") ||
		strings.Contains(msg, "UNAVAILABLE") ||
		strings.Contains(msg, "DEADLINE_EXCEEDED") ||
		strings.Contains(msg, "quota")
}

func (s GCPRetryStrategy) Backoff(attempt int) time.Duration {
	// Exponential: 1s * 2^(attempt-1), capped at 32s.
	d := time.Second * (1 << uint(attempt-1))
	if d > 32*time.Second {
		d = 32 * time.Second
	}
	return d
}

// AzureRetryStrategy handles Azure OpenAI / Cognitive Services transient errors.
// Respects the recommended Retry-After window for 429 / rate limit responses.
type AzureRetryStrategy struct {
	MaxAttempts int
}

func (s AzureRetryStrategy) ShouldRetry(attempt int, err error) bool {
	max := s.MaxAttempts
	if max <= 0 {
		max = 4
	}
	if attempt > max || err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "429") ||
		strings.Contains(msg, "RateLimitError") ||
		strings.Contains(msg, "ServiceBusy") ||
		strings.Contains(msg, "rate limit")
}

func (s AzureRetryStrategy) Backoff(attempt int) time.Duration {
	// Fixed formula: 20s base + 10s per attempt. Caller cannot read Retry-After
	// from the HTTP response through this interface, so we use a conservative default.
	return 20*time.Second + time.Duration(attempt)*10*time.Second
}

// ProviderRetryStrategy returns the appropriate RetryStrategy for a cloud provider.
// Falls back to DefaultRetryStrategy for unknown providers.
func ProviderRetryStrategy(provider string) RetryStrategy {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "aws-lambda":
		return AWSRetryStrategy{}
	case "gcp-functions":
		return GCPRetryStrategy{}
	case "azure-functions":
		return AzureRetryStrategy{}
	default:
		return DefaultRetryStrategy{}
	}
}

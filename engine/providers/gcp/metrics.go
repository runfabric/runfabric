package gcp

import (
	"context"

	"github.com/runfabric/runfabric/engine/internal/config"
)

// FunctionMetrics holds per-function metrics (compatible shape with AWS Lambda metrics for CLI output).
type FunctionMetrics struct {
	Invocations *float64 `json:"invocations,omitempty"`
	Errors      *float64 `json:"errors,omitempty"`
	DurationAvg *float64 `json:"durationAvgMs,omitempty"`
}

// FetchMetrics returns metrics for Cloud Functions. Currently returns empty map; Cloud Monitoring API can be wired for invocations/errors/duration.
func FetchMetrics(ctx context.Context, cfg *config.Config, stage string) (map[string]FunctionMetrics, error) {
	_ = ctx
	_ = cfg
	_ = stage
	return nil, nil
}

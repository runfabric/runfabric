package aws

import (
	"context"
	"fmt"

	coreconfig "github.com/runfabric/runfabric/platform/core/model/config"
	"github.com/runfabric/runfabric/platform/core/state/transactions"
	"github.com/runfabric/runfabric/platform/core/workflow/recovery"

	awstarget "github.com/runfabric/runfabric/platform/extensions/internal/providers/aws"
)

// New returns the built-in AWS provider implementation.
func New() *awstarget.Provider {
	return awstarget.New()
}

// LambdaMetrics exposes provider metrics in a stable shape for app/workflow callers.
type LambdaMetrics struct {
	Invocations *float64 `json:"invocations,omitempty"`
	Errors      *float64 `json:"errors,omitempty"`
	DurationAvg *float64 `json:"durationAvgMs,omitempty"`
}

// XRayTraceSummary exposes trace metadata in a stable shape for app/workflow callers.
type XRayTraceSummary struct {
	ID           string  `json:"id,omitempty"`
	Duration     float64 `json:"duration,omitempty"`
	ResponseTime float64 `json:"responseTime,omitempty"`
	HTTPStatus   *int32  `json:"httpStatus,omitempty"`
	ServiceCount *int    `json:"serviceCount,omitempty"`
	HasError     *bool   `json:"hasError,omitempty"`
}

// DevStreamState holds state for redirecting API Gateway to a tunnel and restoring on exit.
type DevStreamState = awstarget.DevStreamState

// FetchLambdaMetrics returns CloudWatch metrics for Lambda functions in the config, for the last hour.
func FetchLambdaMetrics(ctx context.Context, cfg *coreconfig.Config, stage string) (map[string]LambdaMetrics, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config required")
	}
	// Call internal implementation
	metrics, err := awstarget.FetchLambdaMetrics(ctx, cfg, stage)
	if err != nil {
		return nil, err
	}

	// Convert engine types to public interface types
	result := make(map[string]LambdaMetrics)
	for k, v := range metrics {
		result[k] = LambdaMetrics{
			Invocations: v.Invocations,
			Errors:      v.Errors,
			DurationAvg: v.DurationAvg,
		}
	}
	return result, nil
}

// FetchXRayTraces returns trace summaries from AWS X-Ray for the last hour.
func FetchXRayTraces(ctx context.Context, cfg *coreconfig.Config, stage string) ([]XRayTraceSummary, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config required")
	}
	// Call internal implementation
	traces, err := awstarget.FetchXRayTraces(ctx, cfg, stage)
	if err != nil {
		return nil, err
	}

	// Convert engine types to public interface types
	result := make([]XRayTraceSummary, len(traces))
	for i, t := range traces {
		result[i] = XRayTraceSummary{
			ID:           t.ID,
			Duration:     t.Duration,
			ResponseTime: t.ResponseTime,
			HTTPStatus:   t.HTTPStatus,
			ServiceCount: t.ServiceCount,
			HasError:     t.HasError,
		}
	}
	return result, nil
}

// RedirectToTunnel finds the HTTP API for the service/stage and redirects it to the tunnel.
func RedirectToTunnel(ctx context.Context, cfg *coreconfig.Config, stage, tunnelURL string) (*DevStreamState, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config required")
	}
	return awstarget.RedirectToTunnel(ctx, cfg, stage, tunnelURL)
}

// ResumeDeploy resumes a deployment from a journal checkpoint.
func ResumeDeploy(ctx context.Context, cfg *coreconfig.Config, stage, root string, jf *transactions.JournalFile) (map[string]any, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config required")
	}
	// Call internal implementation
	res, err := awstarget.ResumeDeploy(ctx, cfg, stage, root, jf)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, nil
	}
	return *res, nil
}

// NewRecoveryHandler returns the AWS recovery handler for rollback.
func NewRecoveryHandler(journal *transactions.JournalFile) recovery.Handler {
	return awstarget.NewRecoveryHandler(journal)
}

// Conversion helpers (temporary until Phase 3 migrates internal to platform/ paths)

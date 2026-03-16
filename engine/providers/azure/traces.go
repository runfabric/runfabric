package azure

import (
	"context"

	"github.com/runfabric/runfabric/engine/internal/config"
)

// TraceSummary is a simplified trace summary for CLI output (compatible shape with AWS X-Ray).
type TraceSummary struct {
	ID           string  `json:"id,omitempty"`
	Duration     float64 `json:"duration,omitempty"`
	ResponseTime float64 `json:"responseTime,omitempty"`
	HTTPStatus   *int32  `json:"httpStatus,omitempty"`
	ServiceCount *int    `json:"serviceCount,omitempty"`
	HasError     *bool   `json:"hasError,omitempty"`
}

// FetchTraces returns trace summaries from Application Insights. Currently returns empty slice; Application Insights API can be wired.
func FetchTraces(ctx context.Context, cfg *config.Config, stage string) ([]TraceSummary, error) {
	_ = ctx
	_ = cfg
	_ = stage
	return nil, nil
}

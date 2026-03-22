package aws

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/xray"
	xraytypes "github.com/aws/aws-sdk-go-v2/service/xray/types"
	"github.com/runfabric/runfabric/platform/core/model/config"
)

// XRayTraceSummary is a simplified trace summary for CLI output.
type XRayTraceSummary struct {
	ID           string  `json:"id,omitempty"`
	Duration     float64 `json:"duration,omitempty"`
	ResponseTime float64 `json:"responseTime,omitempty"`
	HTTPStatus   *int32  `json:"httpStatus,omitempty"`
	ServiceCount *int    `json:"serviceCount,omitempty"`
	HasError     *bool   `json:"hasError,omitempty"`
}

// FetchXRayTraces returns trace summaries from AWS X-Ray for the last hour, optionally filtered by service name.
func FetchXRayTraces(ctx context.Context, cfg *config.Config, stage string) ([]XRayTraceSummary, error) {
	clients, err := loadClients(ctx, cfg.Provider.Region)
	if err != nil {
		return nil, err
	}
	end := time.Now()
	start := end.Add(-1 * time.Hour)
	var filter *string
	if cfg.Service != "" {
		expr := "service(\"" + cfg.Service + "\")"
		filter = &expr
	}
	input := &xray.GetTraceSummariesInput{
		StartTime:     aws.Time(start),
		EndTime:       aws.Time(end),
		TimeRangeType: xraytypes.TimeRangeTypeTraceId,
	}
	if filter != nil {
		input.FilterExpression = filter
	}
	out, err := clients.XRay.GetTraceSummaries(ctx, input)
	if err != nil {
		return nil, err
	}
	summaries := make([]XRayTraceSummary, 0, len(out.TraceSummaries))
	for _, s := range out.TraceSummaries {
		summary := XRayTraceSummary{}
		if s.Id != nil {
			summary.ID = *s.Id
		}
		if s.Duration != nil {
			summary.Duration = *s.Duration
		}
		if s.ResponseTime != nil {
			summary.ResponseTime = *s.ResponseTime
		}
		if s.Http != nil {
			summary.HTTPStatus = s.Http.HttpStatus
		}
		if s.ServiceIds != nil && len(s.ServiceIds) > 0 {
			n := len(s.ServiceIds)
			summary.ServiceCount = &n
		}
		summary.HasError = s.HasError
		summaries = append(summaries, summary)
	}
	return summaries, nil
}

package aws

import (
	"context"
	"fmt"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	cloudwatchv2 "github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cloudwatchtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	xrayv2 "github.com/aws/aws-sdk-go-v2/service/xray"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// metricsWindow is how far back per-function metrics are aggregated.
const metricsWindow = time.Hour

// FetchMetrics returns real CloudWatch AWS/Lambda metrics (invocations, errors,
// average duration ms) per configured function over the last hour.
func (p *Provider) FetchMetrics(ctx context.Context, req sdkprovider.MetricsRequest) (*sdkprovider.MetricsResult, error) {
	cfg := req.Config
	stage := req.Stage
	service := sdkprovider.Service(cfg)
	region := sdkprovider.ProviderRegion(cfg)
	if region == "" {
		region = sdkprovider.Env("AWS_REGION")
	}
	clients, err := loadClients(ctx, region)
	if err != nil {
		return nil, fmt.Errorf("load aws clients: %w", err)
	}

	end := time.Now()
	start := end.Add(-metricsWindow)
	perFn := map[string]any{}
	for fn := range sdkprovider.Functions(cfg) {
		functionName := fmt.Sprintf("%s-%s-%s", service, stage, fn)
		perFn[fn] = map[string]any{
			"invocations": lambdaMetric(ctx, clients, functionName, "Invocations", cloudwatchtypes.StatisticSum, start, end),
			"errors":      lambdaMetric(ctx, clients, functionName, "Errors", cloudwatchtypes.StatisticSum, start, end),
			"durationMs":  lambdaMetric(ctx, clients, functionName, "Duration", cloudwatchtypes.StatisticAverage, start, end),
		}
	}
	return &sdkprovider.MetricsResult{
		PerFunction: perFn,
		Message:     "CloudWatch AWS/Lambda metrics (last 1h).",
	}, nil
}

// lambdaMetric reads one AWS/Lambda metric for a function; returns 0 on any
// error (a not-yet-reporting function has no datapoints, not a failure).
func lambdaMetric(ctx context.Context, clients *AWSClients, functionName, metric string, stat cloudwatchtypes.Statistic, start, end time.Time) float64 {
	out, err := clients.CloudWatch.GetMetricStatistics(ctx, &cloudwatchv2.GetMetricStatisticsInput{
		Namespace:  awssdk.String("AWS/Lambda"),
		MetricName: awssdk.String(metric),
		Dimensions: []cloudwatchtypes.Dimension{{
			Name:  awssdk.String("FunctionName"),
			Value: awssdk.String(functionName),
		}},
		StartTime:  awssdk.Time(start),
		EndTime:    awssdk.Time(end),
		Period:     awssdk.Int32(int32(metricsWindow.Seconds())),
		Statistics: []cloudwatchtypes.Statistic{stat},
	})
	if err != nil || out == nil {
		return 0
	}
	var val float64
	for _, dp := range out.Datapoints {
		switch stat {
		case cloudwatchtypes.StatisticSum:
			if dp.Sum != nil {
				val += *dp.Sum
			}
		case cloudwatchtypes.StatisticAverage:
			if dp.Average != nil {
				val = *dp.Average
			}
		}
	}
	return val
}

// FetchTraces returns recent X-Ray trace summaries (id, duration, response
// time) for the last hour. X-Ray is region-wide, not per-function; when X-Ray
// is unavailable (e.g. an emulator without it) the error is surfaced in the
// message and the trace list is empty rather than failing the caller.
func (p *Provider) FetchTraces(ctx context.Context, req sdkprovider.TracesRequest) (*sdkprovider.TracesResult, error) {
	region := sdkprovider.ProviderRegion(req.Config)
	if region == "" {
		region = sdkprovider.Env("AWS_REGION")
	}
	clients, err := loadClients(ctx, region)
	if err != nil {
		return nil, fmt.Errorf("load aws clients: %w", err)
	}

	end := time.Now()
	start := end.Add(-metricsWindow)
	out, err := clients.XRay.GetTraceSummaries(ctx, &xrayv2.GetTraceSummariesInput{
		StartTime: awssdk.Time(start),
		EndTime:   awssdk.Time(end),
	})
	if err != nil {
		return &sdkprovider.TracesResult{
			Traces:  []any{},
			Message: fmt.Sprintf("X-Ray traces unavailable: %v", err),
		}, nil
	}

	traces := make([]any, 0, len(out.TraceSummaries))
	for _, s := range out.TraceSummaries {
		traces = append(traces, map[string]any{
			"id":              awssdk.ToString(s.Id),
			"durationSec":     awssdk.ToFloat64(s.Duration),
			"responseTimeSec": awssdk.ToFloat64(s.ResponseTime),
		})
	}
	return &sdkprovider.TracesResult{
		Traces:  traces,
		Message: fmt.Sprintf("X-Ray trace summaries (last 1h): %d", len(traces)),
	}, nil
}

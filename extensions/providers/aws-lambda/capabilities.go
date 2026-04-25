package aws

import (
	"context"
	"fmt"
	"strings"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

func (p *Provider) FetchMetrics(ctx context.Context, req sdkprovider.MetricsRequest) (*sdkprovider.MetricsResult, error) {
	_ = ctx
	out := map[string]any{}
	for fn := range sdkprovider.Functions(req.Config) {
		out[fn] = map[string]any{"status": "available-via-cloudwatch"}
	}
	return &sdkprovider.MetricsResult{
		PerFunction: out,
		Message:     "AWS metrics are available via CloudWatch.",
	}, nil
}

func (p *Provider) FetchTraces(ctx context.Context, req sdkprovider.TracesRequest) (*sdkprovider.TracesResult, error) {
	_ = ctx
	traces := []any{}
	for fn := range sdkprovider.Functions(req.Config) {
		traces = append(traces, map[string]any{"function": fn, "status": "available-via-xray"})
	}
	return &sdkprovider.TracesResult{
		Traces:  traces,
		Message: "AWS traces are available via X-Ray.",
	}, nil
}

func (p *Provider) PrepareDevStream(ctx context.Context, req sdkprovider.DevStreamRequest) (*sdkprovider.DevStreamSession, error) {
	_ = strings.TrimSpace(req.Region)
	return sdkprovider.PrepareLifecycleDevStream(ProviderID, req.Config, req.Stage, req.TunnelURL)
}

func (p *Provider) Recover(ctx context.Context, req sdkprovider.RecoveryRequest) (*sdkprovider.RecoveryResult, error) {
	_ = ctx
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	switch mode {
	case "resume":
		return &sdkprovider.RecoveryResult{
			Recovered:  true,
			Mode:       "resume",
			Status:     "resumed",
			Message:    "aws recovery resume acknowledged",
			ResumeData: map[string]any{"status": "accepted"},
		}, nil
	case "rollback", "inspect":
		return &sdkprovider.RecoveryResult{
			Recovered: mode == "rollback",
			Mode:      mode,
			Status:    "inspected",
			Message:   "aws recovery inspection completed",
			Metadata: map[string]string{
				"service": req.Service,
				"stage":   req.Stage,
			},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported recovery mode %q", req.Mode)
	}
}

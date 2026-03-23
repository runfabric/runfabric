package aws

import (
	"context"
	"fmt"
	"strings"

	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	"github.com/runfabric/runfabric/platform/core/state/transactions"
	"github.com/runfabric/runfabric/platform/core/workflow/recovery"
)

func (p *Provider) FetchMetrics(ctx context.Context, req providers.MetricsRequest) (*providers.MetricsResult, error) {
	if req.Config == nil {
		return &providers.MetricsResult{Message: "Metrics: use provider console when config is unavailable."}, nil
	}
	cloudMetrics, err := FetchLambdaMetrics(ctx, req.Config, req.Stage)
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	for fn, m := range cloudMetrics {
		out[fn] = m
	}
	if len(out) == 0 {
		return &providers.MetricsResult{Message: "Metrics: use provider console (e.g. CloudWatch) when not deployed or region unavailable."}, nil
	}
	return &providers.MetricsResult{
		PerFunction: out,
		Message:     "CloudWatch metrics (last 1h); use provider console for more.",
	}, nil
}

func (p *Provider) FetchTraces(ctx context.Context, req providers.TracesRequest) (*providers.TracesResult, error) {
	if req.Config == nil {
		return &providers.TracesResult{Message: "Traces: use provider console when config is unavailable."}, nil
	}
	summaries, err := FetchXRayTraces(ctx, req.Config, req.Stage)
	if err != nil {
		return nil, err
	}
	traces := make([]any, 0, len(summaries))
	for _, summary := range summaries {
		traces = append(traces, summary)
	}
	if len(traces) == 0 {
		return &providers.TracesResult{Message: "Traces: use provider console or runfabric logs when X-Ray is unavailable."}, nil
	}
	return &providers.TracesResult{
		Traces:  traces,
		Message: "X-Ray trace summaries (last 1h); use AWS console for full trace details.",
	}, nil
}

func (p *Provider) PrepareDevStream(ctx context.Context, req providers.DevStreamRequest) (*providers.DevStreamSession, error) {
	state, err := RedirectToTunnel(ctx, req.Config, req.Stage, req.TunnelURL)
	if err != nil {
		return nil, err
	}
	if state == nil {
		return nil, nil
	}
	region := strings.TrimSpace(req.Region)
	if region == "" && req.Config != nil {
		region = strings.TrimSpace(req.Config.Provider.Region)
	}
	return providers.NewDevStreamSession(
		"route-rewrite",
		nil,
		"full route rewrite configured; provider state will be restored on exit",
		func(restoreCtx context.Context) error {
			return state.Restore(restoreCtx, region)
		},
	), nil
}

func (p *Provider) Recover(ctx context.Context, req providers.RecoveryRequest) (*providers.RecoveryResult, error) {
	journal, _ := req.Journal.(*transactions.JournalFile)
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	switch mode {
	case "resume":
		if req.Config == nil {
			return nil, fmt.Errorf("config required")
		}
		res, err := ResumeDeploy(ctx, req.Config, req.Stage, req.Root, journal)
		if err != nil {
			return nil, err
		}
		resumeData := map[string]any{}
		if res != nil {
			resumeData = map[string]any(*res)
		}
		return &providers.RecoveryResult{
			Recovered:  true,
			Mode:       "resume",
			Status:     "resumed",
			Message:    "aws resume completed",
			ResumeData: resumeData,
		}, nil
	case "rollback", "inspect":
		handler := NewRecoveryHandler(journal)
		r := recovery.Request{Root: req.Root, Service: req.Service, Stage: req.Stage, Region: req.Region}
		var (
			out *recovery.Result
			err error
		)
		if mode == "rollback" {
			out, err = handler.Rollback(ctx, r)
		} else {
			out, err = handler.Inspect(ctx, r)
		}
		if err != nil {
			return nil, err
		}
		if out == nil {
			return nil, fmt.Errorf("aws recovery %q returned no result", mode)
		}
		return &providers.RecoveryResult{
			Recovered: out.Recovered,
			Mode:      out.Mode,
			Status:    out.Status,
			Message:   out.Message,
			Metadata:  out.Metadata,
			Errors:    out.Errors,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported recovery mode %q", req.Mode)
	}
}

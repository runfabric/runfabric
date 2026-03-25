package azure

import (
	"context"
	"fmt"
	"strings"

	"github.com/runfabric/runfabric/platform/core/state/transactions"
	"github.com/runfabric/runfabric/platform/workflow/recovery"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

func PrepareDevStreamPolicy(ctx context.Context, cfg sdkprovider.Config, stage, tunnelURL string) (*sdkprovider.DevStreamSession, error) {
	state, err := RedirectToTunnel(ctx, cfg, stage, tunnelURL)
	if err != nil {
		return nil, err
	}
	if state == nil {
		return nil, nil
	}
	return &sdkprovider.DevStreamSession{EffectiveMode: state.Mode, MissingPrereqs: append([]string(nil), state.MissingPrereqs...), StatusMessage: state.StatusMessage}, nil
}

func FetchMetricsPolicy(ctx context.Context, cfg sdkprovider.Config, stage string) (*sdkprovider.MetricsResult, error) {
	perFn, err := FetchMetrics(ctx, cfg, stage)
	if err != nil {
		return nil, err
	}
	out := make(map[string]any, len(perFn))
	for fn, m := range perFn {
		out[fn] = m
	}
	if len(out) == 0 {
		return &sdkprovider.MetricsResult{Message: "Azure: use Azure Portal / Application Insights for function metrics."}, nil
	}
	return &sdkprovider.MetricsResult{PerFunction: out, Message: "Azure Application Insights metrics; use Azure Portal for more."}, nil
}

func FetchTracesPolicy(ctx context.Context, cfg sdkprovider.Config, stage string) (*sdkprovider.TracesResult, error) {
	summaries, err := FetchTraces(ctx, cfg, stage)
	if err != nil {
		return nil, err
	}
	traces := make([]any, 0, len(summaries))
	for _, s := range summaries {
		traces = append(traces, s)
	}
	if len(traces) == 0 {
		return &sdkprovider.TracesResult{Message: "Azure: use Azure Portal / Application Insights for traces."}, nil
	}
	return &sdkprovider.TracesResult{Traces: traces, Message: "Azure Application Insights traces; use Azure Portal for full details."}, nil
}

func RecoverPolicy(ctx context.Context, req sdkprovider.RecoveryRequest) (*sdkprovider.RecoveryResult, error) {
	journal, _ := req.Journal.(*transactions.JournalFile)
	handler := NewRecoveryHandler(journal)
	recoveryReq := recovery.Request{
		Root:    req.Root,
		Service: req.Service,
		Stage:   req.Stage,
		Region:  req.Region,
	}
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	var (
		out *recovery.Result
		err error
	)
	switch mode {
	case "rollback":
		out, err = handler.Rollback(ctx, recoveryReq)
	case "resume":
		out, err = handler.Resume(ctx, recoveryReq)
	case "inspect":
		out, err = handler.Inspect(ctx, recoveryReq)
	default:
		return nil, fmt.Errorf("unsupported recovery mode %q", req.Mode)
	}
	if err != nil {
		return nil, err
	}
	if out == nil {
		return nil, fmt.Errorf("azure recovery %q returned no result", mode)
	}
	return &sdkprovider.RecoveryResult{
		Recovered: out.Recovered,
		Mode:      out.Mode,
		Status:    out.Status,
		Message:   out.Message,
		Metadata:  out.Metadata,
		Errors:    out.Errors,
	}, nil
}

func SyncOrchestrationsPolicy(ctx context.Context, req sdkprovider.OrchestrationSyncRequest) (*sdkprovider.OrchestrationSyncResult, error) {
	return (Runner{}).SyncOrchestrations(ctx, req)
}

func RemoveOrchestrationsPolicy(ctx context.Context, req sdkprovider.OrchestrationRemoveRequest) (*sdkprovider.OrchestrationSyncResult, error) {
	return (Runner{}).RemoveOrchestrations(ctx, req)
}

func InvokeOrchestrationPolicy(ctx context.Context, req sdkprovider.OrchestrationInvokeRequest) (*sdkprovider.InvokeResult, error) {
	return (Runner{}).InvokeOrchestration(ctx, req)
}

func InspectOrchestrationsPolicy(ctx context.Context, req sdkprovider.OrchestrationInspectRequest) (map[string]any, error) {
	return (Runner{}).InspectOrchestrations(ctx, req)
}

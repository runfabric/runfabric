package azure

import (
	"context"
	"fmt"
	"strings"

	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	"github.com/runfabric/runfabric/platform/core/state/transactions"
	"github.com/runfabric/runfabric/platform/core/workflow/recovery"
)

func PrepareDevStreamPolicy(ctx context.Context, cfg *providers.Config, stage, tunnelURL string) (*providers.DevStreamSession, error) {
	state, err := RedirectToTunnel(ctx, cfg, stage, tunnelURL)
	if err != nil {
		return nil, err
	}
	if state == nil {
		return nil, nil
	}
	return providers.NewDevStreamSession(state.Mode, state.MissingPrereqs, state.StatusMessage, func(restoreCtx context.Context) error {
		return state.Restore(restoreCtx)
	}), nil
}

func FetchMetricsPolicy(ctx context.Context, cfg *providers.Config, stage string) (*providers.MetricsResult, error) {
	perFn, err := FetchMetrics(ctx, cfg, stage)
	if err != nil {
		return nil, err
	}
	out := make(map[string]any, len(perFn))
	for fn, m := range perFn {
		out[fn] = m
	}
	if len(out) == 0 {
		return &providers.MetricsResult{Message: "Azure: use Azure Portal / Application Insights for function metrics."}, nil
	}
	return &providers.MetricsResult{PerFunction: out, Message: "Azure Application Insights metrics; use Azure Portal for more."}, nil
}

func FetchTracesPolicy(ctx context.Context, cfg *providers.Config, stage string) (*providers.TracesResult, error) {
	summaries, err := FetchTraces(ctx, cfg, stage)
	if err != nil {
		return nil, err
	}
	traces := make([]any, 0, len(summaries))
	for _, s := range summaries {
		traces = append(traces, s)
	}
	if len(traces) == 0 {
		return &providers.TracesResult{Message: "Azure: use Azure Portal / Application Insights for traces."}, nil
	}
	return &providers.TracesResult{Traces: traces, Message: "Azure Application Insights traces; use Azure Portal for full details."}, nil
}

func RecoverPolicy(ctx context.Context, req providers.RecoveryRequest) (*providers.RecoveryResult, error) {
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
	return &providers.RecoveryResult{
		Recovered: out.Recovered,
		Mode:      out.Mode,
		Status:    out.Status,
		Message:   out.Message,
		Metadata:  out.Metadata,
		Errors:    out.Errors,
	}, nil
}

func SyncOrchestrationsPolicy(ctx context.Context, req providers.OrchestrationSyncRequest) (*providers.OrchestrationSyncResult, error) {
	return (Runner{}).SyncOrchestrations(ctx, req)
}

func RemoveOrchestrationsPolicy(ctx context.Context, req providers.OrchestrationRemoveRequest) (*providers.OrchestrationSyncResult, error) {
	return (Runner{}).RemoveOrchestrations(ctx, req)
}

func InvokeOrchestrationPolicy(ctx context.Context, req providers.OrchestrationInvokeRequest) (*providers.InvokeResult, error) {
	return (Runner{}).InvokeOrchestration(ctx, req)
}

func InspectOrchestrationsPolicy(ctx context.Context, req providers.OrchestrationInspectRequest) (map[string]any, error) {
	return (Runner{}).InspectOrchestrations(ctx, req)
}

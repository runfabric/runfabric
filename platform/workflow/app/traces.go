package app

import (
	"context"

	providers "github.com/runfabric/runfabric/internal/provider/contracts"
	state "github.com/runfabric/runfabric/platform/core/state/core"
)

// Traces returns trace data for the deployed service (from receipt/metadata or provider).
// When all is true, output is aggregated by service/stage. For AWS, fetches X-Ray trace summaries when available.
func Traces(configPath, stage, providerOverride string, all bool, service string) (any, error) {
	ctx, err := Bootstrap(configPath, stage, providerOverride)
	if err != nil {
		return nil, err
	}
	if err := validateServiceScope(ctx.Config.Service, service); err != nil {
		return nil, err
	}
	provider, err := resolveProvider(ctx)
	if err != nil {
		return nil, err
	}
	receipt, _ := ctx.Backends.Receipts.Load(ctx.Stage)
	out := map[string]any{
		"provider": provider.name,
		"stage":    ctx.Stage,
		"service":  ctx.Config.Service,
		"traces":   []any{},
	}
	if all {
		out["aggregated"] = "by service/stage"
		out["functionCount"] = len(ctx.Config.Functions)
	}
	if receipt != nil {
		out["deploymentId"] = receipt.DeploymentID
		out["updatedAt"] = receipt.UpdatedAt
		if runs, _ := state.ListWorkflowRuns(ctx.RootDir, ctx.Stage, 10); len(runs) > 0 {
			out["workflowRuns"] = runs
		}
	}
	if obs, ok := provider.provider.(providers.ObservabilityCapable); ok {
		res, obsErr := obs.FetchTraces(context.Background(), providers.TracesRequest{Config: ctx.Config, Stage: ctx.Stage})
		if obsErr == nil && res != nil {
			if len(res.Traces) > 0 {
				out["traces"] = res.Traces
			}
			if res.Message != "" {
				out["message"] = res.Message
				return out, nil
			}
		}
	}
	out["message"] = "Traces: use provider console or runfabric logs for now; trace export coming soon."
	return out, nil
}

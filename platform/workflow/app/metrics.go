package app

import (
	"context"

	providers "github.com/runfabric/runfabric/internal/provider/contracts"
	state "github.com/runfabric/runfabric/platform/core/state/core"
)

// Metrics returns metrics for the deployed service (from receipt/metadata or provider).
// When all is true, output is aggregated by service/stage. For AWS Lambda, fetches CloudWatch metrics (Invocations, Errors, Duration) when available.
func Metrics(configPath, stage, providerOverride string, all bool, service string) (any, error) {
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
		"metrics":  map[string]any{"invocations": nil, "errors": nil, "duration": nil},
	}
	if all {
		out["aggregated"] = "by service/stage"
		out["functionCount"] = len(ctx.Config.Functions)
	}
	if receipt != nil {
		out["deploymentId"] = receipt.DeploymentID
		out["functionCount"] = len(receipt.Functions)
		runs, _ := state.ListWorkflowRuns(ctx.RootDir, ctx.Stage, 50)
		if len(runs) > 0 {
			out["workflowRuns"] = runs
			out["workflowCost"] = state.WorkflowCostFromRuns(runs)
		}
	}
	if obs, ok := provider.provider.(providers.ObservabilityCapable); ok {
		res, obsErr := obs.FetchMetrics(context.Background(), providers.MetricsRequest{Config: ctx.Config, Stage: ctx.Stage})
		if obsErr == nil && res != nil {
			if len(res.PerFunction) > 0 {
				out["perFunction"] = res.PerFunction
			}
			if res.Message != "" {
				out["message"] = res.Message
				return out, nil
			}
		}
	}
	out["message"] = "Metrics: use provider console for now; metrics export coming soon."
	return out, nil
}

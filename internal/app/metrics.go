package app

import "github.com/runfabric/runfabric/internal/state"

// Metrics returns metrics for the deployed service (from receipt/metadata or provider).
// Provider-specific metrics (invocations, errors, duration) can be added later.
func Metrics(configPath, stage, providerName string) (any, error) {
	ctx, err := Bootstrap(configPath, stage)
	if err != nil {
		return nil, err
	}
	if providerName != "" {
		ctx.Config.Provider.Name = providerName
	}
	receipt, _ := state.Load(ctx.RootDir, ctx.Stage)
	out := map[string]any{
		"provider": ctx.Config.Provider.Name,
		"stage":    ctx.Stage,
		"service":  ctx.Config.Service,
		"metrics":  map[string]any{"invocations": nil, "errors": nil, "duration": nil},
		"message":  "Metrics: use provider console (e.g. CloudWatch) for now; metrics export coming soon",
	}
	if receipt != nil {
		out["deploymentId"] = receipt.DeploymentID
		out["functionCount"] = len(receipt.Functions)
	}
	return out, nil
}

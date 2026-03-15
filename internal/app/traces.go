package app

import "github.com/runfabric/runfabric/internal/state"

// Traces returns trace data for the deployed service (from receipt/metadata or provider).
// Provider-specific trace backends can be added later.
func Traces(configPath, stage, providerName string) (any, error) {
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
		"traces":   []any{},
		"message":  "Traces: use provider console or runfabric logs for now; trace export coming soon",
	}
	if receipt != nil {
		out["deploymentId"] = receipt.DeploymentID
		out["updatedAt"] = receipt.UpdatedAt
	}
	return out, nil
}

// Package api performs real deploys using provider REST APIs and SDKs (no CLI).
// Auth via env vars per provider. Part of internal/deploy; see internal/deploy/cli for CLI-based deploy.
package api

import (
	"context"
	"fmt"

	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/extensions/providers"
	"github.com/runfabric/runfabric/engine/internal/state"
)

// Run deploys via the provider's API and returns a DeployResult. Saves receipt to root.
func Run(ctx context.Context, provider string, cfg *config.Config, stage, root string) (*providers.DeployResult, error) {
	p, ok := getProvider(provider)
	if !ok {
		return nil, fmt.Errorf("deploy via API is not supported for unregistered provider %q", provider)
	}
	result, err := p.Deploy(ctx, cfg, stage, root)
	if err != nil {
		return nil, err
	}
	receipt := &state.Receipt{
		Service:      cfg.Service,
		Stage:        stage,
		Provider:     result.Provider,
		DeploymentID: result.DeploymentID,
		Outputs:      result.Outputs,
		Artifacts:    result.Artifacts,
		Metadata:     result.Metadata,
		Functions:    make([]state.FunctionDeployment, 0, len(result.Artifacts)),
	}
	for _, a := range result.Artifacts {
		receipt.Functions = append(receipt.Functions, state.FunctionDeployment{Function: a.Function})
	}
	state.EnrichReceiptWithAiWorkflow(receipt, cfg)
	if err := state.Save(root, receipt); err != nil {
		return nil, err
	}
	return result, nil
}

// HasRunner returns whether the provider has an API-based deploy runner.
func HasRunner(provider string) bool {
	return hasProvider(provider)
}

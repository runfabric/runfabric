// Package api performs real deploys using provider REST APIs and SDKs (no CLI).
// Auth via env vars per provider. Part of internal/deploy; see internal/deploy/cli for CLI-based deploy.
package api

import (
	"context"
	"fmt"

	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	"github.com/runfabric/runfabric/platform/core/model/config"
	state "github.com/runfabric/runfabric/platform/core/state/core"
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
	artifacts := make([]state.Artifact, 0, len(result.Artifacts))
	for _, a := range result.Artifacts {
		artifacts = append(artifacts, state.Artifact{
			Function:        a.Function,
			Runtime:         a.Runtime,
			SourcePath:      a.SourcePath,
			OutputPath:      a.OutputPath,
			SHA256:          a.SHA256,
			SizeBytes:       a.SizeBytes,
			ConfigSignature: a.ConfigSignature,
		})
	}
	receipt := &state.Receipt{
		Service:      cfg.Service,
		Stage:        stage,
		Provider:     result.Provider,
		DeploymentID: result.DeploymentID,
		Outputs:      result.Outputs,
		Artifacts:    artifacts,
		Metadata:     result.Metadata,
		Functions:    make([]state.FunctionDeployment, 0, len(result.Artifacts)),
	}
	for _, a := range result.Artifacts {
		fn := state.FunctionDeployment{Function: a.Function}
		if deployed, ok := result.Functions[a.Function]; ok {
			fn.ResourceName = deployed.ResourceName
			fn.ResourceIdentifier = deployed.ResourceIdentifier
			fn.Metadata = deployed.Metadata
		}
		receipt.Functions = append(receipt.Functions, fn)
	}
	state.EnrichReceiptWithWorkflows(receipt, cfg)
	if err := state.Save(root, receipt); err != nil {
		return nil, err
	}
	return result, nil
}

// HasRunner returns whether the provider has an API-based deploy runner.
func HasRunner(provider string) bool {
	return hasProvider(provider)
}

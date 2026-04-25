// Package api performs real deploys using provider REST APIs and SDKs (no CLI).
// Auth via env vars per provider. Part of internal/deploy; see internal/deploy/cli for CLI-based deploy.
package api

import (
	"context"
	"fmt"

	providers "github.com/runfabric/runfabric/internal/provider/contracts"
	"github.com/runfabric/runfabric/platform/core/model/config"
)

// Run deploys via the provider's API and returns a DeployResult. Saves receipt to root.
// It computes a Changeset from the last receipt before calling the provider so providers
// can skip unchanged functions and precisely delete removed ones.
func Run(ctx context.Context, provider string, cfg *config.Config, stage, root string) (*providers.DeployResult, error) {
	p, ok := getProvider(provider)
	if !ok {
		return nil, fmt.Errorf("deploy via API is not supported for unregistered provider %q", provider)
	}
	changeset := computeChangeset(cfg, stage, root)
	ctx = providers.ContextWithChangeset(ctx, changeset)
	result, err := p.Deploy(ctx, cfg, stage, root)
	if err != nil {
		return nil, err
	}
	artifacts := make([]ReceiptArtifact, 0, len(result.Artifacts))
	for _, a := range result.Artifacts {
		artifacts = append(artifacts, ReceiptArtifact{
			Function:        a.Function,
			Runtime:         a.Runtime,
			SourcePath:      a.SourcePath,
			OutputPath:      a.OutputPath,
			SHA256:          a.SHA256,
			SizeBytes:       a.SizeBytes,
			ConfigSignature: a.ConfigSignature,
		})
	}
	receipt := &ReceiptRecord{
		Service:      cfg.Service,
		Stage:        stage,
		Provider:     result.Provider,
		DeploymentID: result.DeploymentID,
		Outputs:      result.Outputs,
		Artifacts:    artifacts,
		Metadata:     result.Metadata,
		Functions:    make([]ReceiptFunctionDeployment, 0, len(result.Artifacts)),
	}
	for _, a := range result.Artifacts {
		fn := ReceiptFunctionDeployment{Function: a.Function}
		if deployed, ok := result.Functions[a.Function]; ok {
			fn.ResourceName = deployed.ResourceName
			fn.ResourceIdentifier = deployed.ResourceIdentifier
			fn.Metadata = deployed.Metadata
		}
		receipt.Functions = append(receipt.Functions, fn)
	}
	coreState.EnrichReceiptWithWorkflows(receipt, cfg)
	if err := coreState.SaveReceipt(root, receipt); err != nil {
		return nil, err
	}
	return result, nil
}

// HasRunner returns whether the provider has an API-based deploy runner.
func HasRunner(provider string) bool {
	return hasProvider(provider)
}

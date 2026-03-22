package lifecycle

import (
	"context"

	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	"github.com/runfabric/runfabric/platform/core/model/config"
	state "github.com/runfabric/runfabric/platform/core/state/core"
)

func Deploy(reg *providers.Registry, cfg *config.Config, stage, root string) (*providers.DeployResult, error) {
	p, ok := reg.Get(cfg.Provider.Name)
	if !ok {
		return nil, providers.ErrProviderNotFound(cfg.Provider.Name)
	}

	result, err := p.Deploy(context.Background(), providers.DeployRequest{Config: cfg, Stage: stage, Root: root})
	if err != nil {
		return nil, err
	}

	artifacts := make([]state.Artifact, 0, len(result.Artifacts))
	functions := make([]state.FunctionDeployment, 0, len(result.Artifacts))
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
		fn := state.FunctionDeployment{
			Function:        a.Function,
			ArtifactSHA256:  a.SHA256,
			ConfigSignature: a.ConfigSignature,
		}
		if deployed, ok := result.Functions[a.Function]; ok {
			fn.ResourceName = deployed.ResourceName
			fn.ResourceIdentifier = deployed.ResourceIdentifier
			fn.Metadata = deployed.Metadata
		}
		functions = append(functions, fn)
	}

	receipt := &state.Receipt{
		Service:      cfg.Service,
		Stage:        stage,
		Provider:     result.Provider,
		DeploymentID: result.DeploymentID,
		Outputs:      result.Outputs,
		Artifacts:    artifacts,
		Metadata:     result.Metadata,
		Functions:    functions,
	}
	state.EnrichReceiptWithWorkflows(receipt, cfg)
	if err := state.Save(root, receipt); err != nil {
		return nil, err
	}

	return result, nil
}

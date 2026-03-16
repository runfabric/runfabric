package lifecycle

import (
	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/providers"
	"github.com/runfabric/runfabric/engine/internal/state"
)

func Deploy(reg *providers.Registry, cfg *config.Config, stage, root string) (*providers.DeployResult, error) {
	p, err := reg.Get(cfg.Provider.Name)
	if err != nil {
		return nil, err
	}

	result, err := p.Deploy(cfg, stage, root)
	if err != nil {
		return nil, err
	}

	functions := make([]state.FunctionDeployment, 0, len(result.Artifacts))
	for _, a := range result.Artifacts {
		fn := state.FunctionDeployment{
			Function:        a.Function,
			ArtifactSHA256:  a.SHA256,
			ConfigSignature: a.ConfigSignature,
		}
		if result.Metadata != nil {
			fn.LambdaName = result.Metadata["lambda:"+a.Function+":name"]
			fn.LambdaARN = result.Metadata["lambda:"+a.Function+":arn"]
		}
		functions = append(functions, fn)
	}

	receipt := &state.Receipt{
		Service:      cfg.Service,
		Stage:        stage,
		Provider:     result.Provider,
		DeploymentID: result.DeploymentID,
		Outputs:      result.Outputs,
		Artifacts:    result.Artifacts,
		Metadata:     result.Metadata,
		Functions:    functions,
	}
	if err := state.Save(root, receipt); err != nil {
		return nil, err
	}

	return result, nil
}

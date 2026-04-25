package pipeline

import (
	"context"

	providers "github.com/runfabric/runfabric/internal/provider/contracts"
	deployusecase "github.com/runfabric/runfabric/platform/workflow/usecase/deploy"
)

// DeployStep runs the provider deploy and stores the result in StepContext.DeployResult.
type DeployStep struct {
	Input        deployusecase.Input
	Dependencies deployusecase.Dependencies
}

func (s DeployStep) Name() string { return "deploy" }

func (s DeployStep) Run(ctx context.Context, sc *StepContext) error {
	result, err := deployusecase.Execute(ctx, s.Input, s.Dependencies)
	if err != nil {
		return err
	}
	if dr, ok := result.(*providers.DeployResult); ok {
		sc.DeployResult = dr
	}
	return nil
}

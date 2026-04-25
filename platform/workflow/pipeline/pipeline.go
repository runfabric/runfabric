// Package pipeline provides a composable, sequential post-deploy step runner.
//
// Each Step receives a shared StepContext that carries the config, deploy result,
// and any outputs accumulated by previous steps. Steps are run in order; a
// non-nil error from any step halts the pipeline.
//
// Usage:
//
//	p := pipeline.New(DeployStep{...}, HealthCheckStep{...}, DNSSyncStep{...})
//	if err := p.Run(ctx, &pipeline.StepContext{Config: cfg, Stage: stage}); err != nil {
//	    return nil, err
//	}
package pipeline

import (
	"context"
	"fmt"

	providers "github.com/runfabric/runfabric/internal/provider/contracts"
	"github.com/runfabric/runfabric/platform/core/model/config"
)

// StepContext is shared across all steps in a pipeline run.
type StepContext struct {
	Config      *config.Config
	Stage       string
	RootDir     string
	DeployResult *providers.DeployResult // populated by DeployStep; read by later steps
}

// Step is one unit of work in a deploy pipeline.
type Step interface {
	Name() string
	Run(ctx context.Context, sc *StepContext) error
}

// Pipeline runs a fixed sequence of Steps in order.
type Pipeline struct {
	steps []Step
}

// New constructs a Pipeline from the given steps.
func New(steps ...Step) *Pipeline {
	return &Pipeline{steps: steps}
}

// Run executes each step in order. It stops and returns the first error.
func (p *Pipeline) Run(ctx context.Context, sc *StepContext) error {
	for _, s := range p.steps {
		if err := s.Run(ctx, sc); err != nil {
			return fmt.Errorf("pipeline step %q: %w", s.Name(), err)
		}
	}
	return nil
}

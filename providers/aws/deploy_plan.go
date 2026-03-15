package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/runfabric/runfabric/internal/config"
	"github.com/runfabric/runfabric/internal/deployexec"
	internalproviders "github.com/runfabric/runfabric/internal/providers"
	"github.com/runfabric/runfabric/internal/state"
	"github.com/runfabric/runfabric/internal/transactions"
	"github.com/runfabric/runfabric/providers"
)

type DeployPlan struct {
	cfg     *config.Config
	stage   string
	root    string
	journal *transactions.Journal
}

func NewDeployPlan(
	cfg *config.Config,
	stage string,
	root string,
	journal *transactions.Journal,
) *DeployPlan {
	return &DeployPlan{
		cfg:     cfg,
		stage:   stage,
		root:    root,
		journal: journal,
	}
}

func (p *DeployPlan) Execute(ctx context.Context) (*providers.DeployResult, error) {
	deps, err := newResumeDependencies(ctx, p.root, p.cfg, p.stage, p.journal)
	if err != nil {
		return nil, err
	}

	engine := newDeployEngine(p.cfg, p.stage, p.root, deps)
	execCtx := &deployexec.Context{
		Root:      p.root,
		Config:    p.cfg,
		Stage:     p.stage,
		Artifacts: map[string]internalproviders.Artifact{},
		Receipt:   deps.Receipt,
		Outputs:   map[string]string{},
		Metadata:  map[string]string{},
	}

	if err := engine.Run(ctx, execCtx); err != nil {
		_ = p.Rollback(ctx)
		return nil, err
	}

	functions := make([]state.FunctionDeployment, 0, len(execCtx.Artifacts))
	for _, a := range execCtx.Artifacts {
		fn := state.FunctionDeployment{
			Function:        a.Function,
			ArtifactSHA256:  a.SHA256,
			ConfigSignature: a.ConfigSignature,
		}
		if execCtx.Metadata != nil {
			fn.LambdaName = execCtx.Metadata["lambda:"+a.Function+":name"]
			fn.LambdaARN = execCtx.Metadata["lambda:"+a.Function+":arn"]
		}
		functions = append(functions, fn)
	}

	receipt := &state.Receipt{
		Service:      p.cfg.Service,
		Stage:        p.stage,
		Provider:     "aws",
		DeploymentID: fmt.Sprintf("aws-%s-%d", p.stage, time.Now().Unix()),
		Outputs:      execCtx.Outputs,
		Artifacts:    artifactsFromMap(execCtx.Artifacts),
		Metadata:     execCtx.Metadata,
		Functions:    functions,
	}

	if err := state.Save(p.root, receipt); err != nil {
		return nil, err
	}

	return &providers.DeployResult{
		Service: p.cfg.Service,
		Stage:   p.stage,
	}, nil
}

func (p *DeployPlan) Rollback(ctx context.Context) error {
	// Phase engine has no formal rollback; journal is used for recovery. No-op here.
	return nil
}

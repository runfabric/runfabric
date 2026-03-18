package app

import (
	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/lifecycle"
)

func Plan(configPath, stage, providerOverride string) (any, error) {
	ctx, err := Bootstrap(configPath, stage, providerOverride)
	if err != nil {
		return nil, err
	}
	planResult, err := lifecycle.Plan(ctx.Registry, ctx.Config, ctx.Stage, ctx.RootDir)
	if err != nil {
		return nil, err
	}

	// Phase 14.3: if aiWorkflow is enabled, compile and include graph metadata alongside the infra plan.
	if ctx.Config.AiWorkflow == nil || !ctx.Config.AiWorkflow.Enable {
		return planResult, nil
	}
	g, err := config.CompileAiWorkflow(ctx.Config.AiWorkflow)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"plan": planResult,
		"aiWorkflow": map[string]any{
			"entrypoint": g.Entrypoint,
			"hash":       g.Hash,
			"order":      g.Order,
			"levels":     g.Levels,
			"edges":      g.Edges,
		},
	}, nil
}

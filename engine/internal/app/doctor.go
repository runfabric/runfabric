package app

import (
	"fmt"
	"strings"

	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/diagnostics"
	"github.com/runfabric/runfabric/engine/internal/secrets"
)

func BackendDoctor(configPath, stage string) (any, error) {
	ctx, err := Bootstrap(configPath, stage, "")
	if err != nil {
		return nil, err
	}

	report := &diagnostics.HealthReport{
		Service: ctx.Config.Service,
		Stage:   ctx.Stage,
		Checks:  []diagnostics.CheckResult{},
	}

	// Provider credentials: required env vars per CREDENTIALS.md
	if name := ctx.Config.Provider.Name; name != "" {
		missing := secrets.MissingProviderEnvVars(name)
		if len(missing) == 0 {
			report.Checks = append(report.Checks, diagnostics.CheckResult{
				Name: "provider-credentials", OK: true, Backend: name,
				Message: "required provider env vars set",
			})
		} else {
			report.Checks = append(report.Checks, diagnostics.CheckResult{
				Name: "provider-credentials", OK: false, Backend: name,
				Message: fmt.Sprintf("missing or empty: %s (see docs/CREDENTIALS.md)", strings.Join(missing, ", ")),
			})
		}
	}

	// AI workflow validation + compile (Phase 14.3): surface graph metadata (hash/order/levels) for tooling.
	if ctx.Config.AiWorkflow != nil && ctx.Config.AiWorkflow.Enable {
		g, err := config.CompileAiWorkflow(ctx.Config.AiWorkflow)
		if err != nil {
			report.Checks = append(report.Checks, diagnostics.CheckResult{
				Name:    "ai-workflow",
				OK:      false,
				Backend: "aiflow",
				Message: fmt.Sprintf("compile failed: %v", err),
			})
		} else {
			report.Checks = append(report.Checks, diagnostics.CheckResult{
				Name:    "ai-workflow",
				OK:      true,
				Backend: "aiflow",
				Message: fmt.Sprintf("compiled: entrypoint=%s hash=%s nodes=%d edges=%d levels=%d", g.Entrypoint, g.Hash, len(g.Nodes), len(g.Edges), len(g.Levels)),
			})
		}
	}

	if d, ok := ctx.Backends.Locks.(interface {
		Doctor(service, stage string) diagnostics.CheckResult
	}); ok {
		report.Checks = append(report.Checks, d.Doctor(ctx.Config.Service, ctx.Stage))
	}

	if d, ok := ctx.Backends.Journals.(interface {
		Doctor(service, stage string) diagnostics.CheckResult
	}); ok {
		report.Checks = append(report.Checks, d.Doctor(ctx.Config.Service, ctx.Stage))
	}

	if d, ok := ctx.Backends.Receipts.(interface {
		Doctor(stage string) diagnostics.CheckResult
	}); ok {
		report.Checks = append(report.Checks, d.Doctor(ctx.Stage))
	}

	return report, nil
}

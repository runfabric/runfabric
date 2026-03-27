package app

import (
	"fmt"
	"strings"

	"github.com/runfabric/runfabric/platform/core/policy/secrets"
	"github.com/runfabric/runfabric/platform/observability/diagnostics"
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

func DevStreamDoctor(configPath, stage, tunnelURL string) (any, error) {
	result, err := BackendDoctor(configPath, stage)
	if err != nil {
		return nil, err
	}
	report, ok := result.(*diagnostics.HealthReport)
	if !ok {
		return result, nil
	}
	ctx, err := Bootstrap(configPath, stage, "")
	if err != nil {
		return nil, err
	}
	if err := appendDevStreamChecks(report, ctx.Config.Provider.Name, tunnelURL); err != nil {
		return report, err
	}
	return report, nil
}

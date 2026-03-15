package app

import "github.com/runfabric/runfabric/internal/diagnostics"

func BackendDoctor(configPath, stage string) (any, error) {
	ctx, err := Bootstrap(configPath, stage)
	if err != nil {
		return nil, err
	}

	report := &diagnostics.HealthReport{
		Service: ctx.Config.Service,
		Stage:   ctx.Stage,
		Checks:  []diagnostics.CheckResult{},
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

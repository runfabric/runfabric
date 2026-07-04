package app

import (
	"fmt"
	"os"
	"strings"

	statetypes "github.com/runfabric/runfabric/internal/state/types"
	"github.com/runfabric/runfabric/platform/core/model/config"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy"
	"github.com/runfabric/runfabric/platform/observability/diagnostics"
)

// missingProviderCreds returns the provider-declared required env vars that
// are missing or empty. A var counts as set when either its key or its
// declared mirror spelling is present (e.g. GCP_PROJECT / GCP_PROJECT_ID).
func missingProviderCreds(providerID string) []string {
	var missing []string
	for _, c := range providerpolicy.ProviderCredentials(providerID) {
		if !c.Required {
			continue
		}
		if strings.TrimSpace(os.Getenv(c.EnvKey)) != "" {
			continue
		}
		if c.Mirror != "" && strings.TrimSpace(os.Getenv(c.Mirror)) != "" {
			continue
		}
		missing = append(missing, c.EnvKey)
	}
	return missing
}

// missingStateCreds returns the state backend's declared required env vars
// that are missing, treating the equivalent backend config field as a valid
// substitute (e.g. backend.s3Bucket for RUNFABRIC_S3_BUCKET).
func missingStateCreds(b *config.BackendConfig) []string {
	var missing []string
	for _, c := range providerpolicy.StateBackendCredentials(b.Kind) {
		if !c.Required {
			continue
		}
		envKey := c.EnvKey
		// A custom postgres DSN env name is an alias — either it or the
		// default key satisfies the requirement.
		if b.Kind == "postgres" && strings.TrimSpace(b.PostgresConnectionStringEnv) != "" {
			envKey = strings.TrimSpace(b.PostgresConnectionStringEnv)
		}
		if strings.TrimSpace(os.Getenv(envKey)) != "" {
			continue
		}
		if envKey != c.EnvKey && strings.TrimSpace(os.Getenv(c.EnvKey)) != "" {
			continue
		}
		if stateEnvSatisfiedByConfig(c.EnvKey, b) {
			continue
		}
		missing = append(missing, envKey)
	}
	return missing
}

func stateEnvSatisfiedByConfig(envKey string, b *config.BackendConfig) bool {
	switch envKey {
	case "RUNFABRIC_S3_BUCKET":
		return strings.TrimSpace(b.S3Bucket) != ""
	case "RUNFABRIC_DYNAMODB_TABLE":
		return strings.TrimSpace(b.LockTable) != "" || strings.TrimSpace(b.ReceiptTable) != ""
	case "RUNFABRIC_GCS_BUCKET":
		return strings.TrimSpace(b.GCSBucket) != ""
	case "RUNFABRIC_AZBLOB_CONTAINER":
		return strings.TrimSpace(b.AzblobContainer) != ""
	}
	return false
}

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

	// Provider credentials: required env vars declared by the provider plugin.
	if name := ctx.Config.Provider.Name; name != "" {
		missing := missingProviderCreds(name)
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

	// State-backend credentials: required env vars declared by the backend
	// (a backend config field like backend.s3Bucket satisfies its env var).
	if b := ctx.Config.Backend; b != nil && b.Kind != "" {
		if missing := missingStateCreds(b); len(missing) > 0 {
			report.Checks = append(report.Checks, diagnostics.CheckResult{
				Name: "state-credentials", OK: false, Backend: b.Kind,
				Message: fmt.Sprintf("missing or empty: %s (see docs/CREDENTIALS.md)", strings.Join(missing, ", ")),
			})
		} else if len(providerpolicy.StateBackendCredentials(b.Kind)) > 0 {
			report.Checks = append(report.Checks, diagnostics.CheckResult{
				Name: "state-credentials", OK: true, Backend: b.Kind,
				Message: "required state backend env vars set",
			})
		}
	}

	if d, ok := ctx.Backends.Locks.(interface {
		Doctor(service, stage string) statetypes.CheckResult
	}); ok {
		r := d.Doctor(ctx.Config.Service, ctx.Stage)
		report.Checks = append(report.Checks, diagnostics.CheckResult{
			Name:    r.Name,
			OK:      r.OK,
			Backend: r.Backend,
			Message: r.Message,
		})
	}

	if d, ok := ctx.Backends.Journals.(interface {
		Doctor(service, stage string) statetypes.CheckResult
	}); ok {
		r := d.Doctor(ctx.Config.Service, ctx.Stage)
		report.Checks = append(report.Checks, diagnostics.CheckResult{
			Name:    r.Name,
			OK:      r.OK,
			Backend: r.Backend,
			Message: r.Message,
		})
	}

	if d, ok := ctx.Backends.Receipts.(interface {
		Doctor(stage string) statetypes.CheckResult
	}); ok {
		r := d.Doctor(ctx.Stage)
		report.Checks = append(report.Checks, diagnostics.CheckResult{
			Name:    r.Name,
			OK:      r.OK,
			Backend: r.Backend,
			Message: r.Message,
		})
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

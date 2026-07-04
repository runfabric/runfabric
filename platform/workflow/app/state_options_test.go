package app

import (
	"testing"

	"github.com/runfabric/runfabric/platform/core/model/config"
)

// The default postgres DSN key is ALWAYS honored — a custom
// postgresConnectionStringEnv is an additional alias that wins when set, so
// per-request X-State-Postgres-Url credentials (which target the default key)
// are never silently lost under a renamed env.
func TestBackendOptionsPostgresDSNDefaultFallback(t *testing.T) {
	cfg := &config.Config{
		Service: "svc",
		Backend: &config.BackendConfig{
			Kind:                        "postgres",
			PostgresConnectionStringEnv: "MY_CUSTOM_DSN",
		},
	}

	t.Setenv("MY_CUSTOM_DSN", "")
	t.Setenv("RUNFABRIC_STATE_POSTGRES_URL", "postgres://default/db")
	opts := backendOptionsFromConfigAndEnv(cfg, t.TempDir(), "")
	if opts.PostgresDSN != "postgres://default/db" {
		t.Errorf("default key must be honored when custom alias is unset, got %q", opts.PostgresDSN)
	}

	t.Setenv("MY_CUSTOM_DSN", "postgres://custom/db")
	opts = backendOptionsFromConfigAndEnv(cfg, t.TempDir(), "")
	if opts.PostgresDSN != "postgres://custom/db" {
		t.Errorf("custom alias must win when set, got %q", opts.PostgresDSN)
	}
}

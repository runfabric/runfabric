package codec

import (
	"testing"

	"github.com/runfabric/runfabric/platform/core/model/config"
)

// TestFromCoreConfig_StripsTopLevelSecretStore verifies the central secret store
// is not shipped to plugins, while per-function resolved values are retained.
func TestFromCoreConfig_StripsTopLevelSecretStore(t *testing.T) {
	cfg := &config.Config{
		Service: "svc",
		Secrets: map[string]string{"db_password": "super-secret"},
		Functions: map[string]config.FunctionConfig{
			"api": {
				Handler:     "app.handler",
				Environment: map[string]string{"DB_URL": "postgres://resolved"},
			},
		},
	}

	out, err := FromCoreConfig(cfg)
	if err != nil {
		t.Fatalf("FromCoreConfig: %v", err)
	}
	if _, present := out["Secrets"]; present {
		t.Errorf("top-level secret store must not be sent to plugins: %v", out["Secrets"])
	}
	// The original config must not be mutated.
	if cfg.Secrets["db_password"] != "super-secret" {
		t.Error("FromCoreConfig mutated the source config's Secrets")
	}
	// Per-function resolved environment must still be present (needed for deploy).
	fns, ok := out["Functions"].(map[string]any)
	if !ok {
		t.Fatalf("Functions missing/!map: %T", out["Functions"])
	}
	if _, ok := fns["api"]; !ok {
		t.Error("function 'api' should be present in transport config")
	}
}

package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRouterSyncFromFabricStateRequiresFabricEndpoints(t *testing.T) {
	providerName, runtimeName := testProviderNameAndRuntime(t)

	project := t.TempDir()
	configPath := filepath.Join(project, "runfabric.yml")
	configYAML := []byte(`service: svc
provider:
  name: ` + providerName + `
  runtime: ` + runtimeName + `
functions:
  - name: api
    entry: src/handler.default
`)
	if err := os.WriteFile(configPath, configYAML, 0o644); err != nil {
		t.Fatalf("write runfabric.yml: %v", err)
	}

	// No fabric deploy has run → no fabric state → a clear, actionable error
	// (not a nil-routing panic or a silent no-op sync).
	_, _, err := RouterSyncFromFabricState(configPath, "dev", true, nil)
	if err == nil {
		t.Fatal("expected an error when no fabric endpoints are recorded")
	}
	if !strings.Contains(err.Error(), "no fabric endpoints") {
		t.Fatalf("error should point at the missing fabric deploy, got: %v", err)
	}
}

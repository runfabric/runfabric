package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/runfabric/runfabric/platform/observability/diagnostics"
	"github.com/runfabric/runfabric/platform/workflow/app"
)

func TestBackendDoctorE2ELocal(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "runfabric.yml")
	providerName, runtimeName := testProviderNameAndRuntime(t)

	cfg := "service: integration-svc\n" +
		"provider:\n" +
		"  name: " + providerName + "\n" +
		"  runtime: " + runtimeName + "\n" +
		"  region: ap-southeast-1\n" +
		"backend:\n" +
		"  kind: local\n" +
		"functions:\n" +
		"  - name: hello\n" +
		"    entry: src/handler.hello\n" +
		"    runtime: " + runtimeName + "\n"
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := app.BackendDoctor(cfgPath, "dev")
	if err != nil {
		t.Fatalf("BackendDoctor failed: %v", err)
	}

	report, ok := result.(*diagnostics.HealthReport)
	if !ok {
		t.Fatalf("expected *diagnostics.HealthReport, got %T", result)
	}

	if report.Service != "integration-svc" {
		t.Fatalf("expected service=integration-svc, got %q", report.Service)
	}
	if len(report.Checks) == 0 {
		t.Fatal("expected health checks")
	}
}

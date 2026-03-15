package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/runfabric/runfabric/internal/app"
	"github.com/runfabric/runfabric/internal/diagnostics"
)

func TestBackendDoctorE2ELocal(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "runfabric.yml")

	cfg := `
service: integration-svc
provider:
  name: aws
  runtime: nodejs20.x
  region: ap-southeast-1
backend:
  kind: local
functions:
  hello:
    handler: src/handler.hello
    runtime: nodejs20.x
`
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

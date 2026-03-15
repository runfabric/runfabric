package unit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/runfabric/runfabric/internal/app"
	"github.com/runfabric/runfabric/internal/diagnostics"
)

func TestBackendDoctorLocalHealthy(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := writeBackendDoctorConfig(t, tmp)

	result, err := app.BackendDoctor(cfgPath, "dev")
	if err != nil {
		t.Fatalf("BackendDoctor failed: %v", err)
	}

	report, ok := result.(*diagnostics.HealthReport)
	if !ok {
		t.Fatalf("expected *diagnostics.HealthReport, got %T", result)
	}

	if report.Service != "doctor-svc" {
		t.Fatalf("expected service=doctor-svc, got %q", report.Service)
	}
	if report.Stage != "dev" {
		t.Fatalf("expected stage=dev, got %q", report.Stage)
	}
	if len(report.Checks) == 0 {
		t.Fatal("expected at least one backend check")
	}
}

func TestBackendDoctorIncludesBackendKind(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := writeBackendDoctorConfig(t, tmp)

	result, err := app.BackendDoctor(cfgPath, "dev")
	if err != nil {
		t.Fatalf("BackendDoctor failed: %v", err)
	}

	report := result.(*diagnostics.HealthReport)

	foundBackendKind := false
	for _, check := range report.Checks {
		if check.Backend != "" {
			foundBackendKind = true
			break
		}
	}

	if !foundBackendKind {
		t.Fatal("expected at least one check to include backend kind")
	}
}

func TestBackendDoctorAggregatesChecks(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := writeBackendDoctorConfig(t, tmp)

	result, err := app.BackendDoctor(cfgPath, "dev")
	if err != nil {
		t.Fatalf("BackendDoctor failed: %v", err)
	}

	report := result.(*diagnostics.HealthReport)

	if len(report.Checks) < 1 {
		t.Fatalf("expected >=1 check, got %d", len(report.Checks))
	}

	for _, check := range report.Checks {
		if check.Name == "" {
			t.Fatal("expected every check to have a name")
		}
	}
}

func writeBackendDoctorConfig(t *testing.T, root string) string {
	t.Helper()

	cfg := `
service: doctor-svc
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
	path := filepath.Join(root, "runfabric.yml")
	if err := os.WriteFile(path, []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}
	return path
}

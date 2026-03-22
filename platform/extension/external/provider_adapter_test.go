package external

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	"github.com/runfabric/runfabric/platform/core/model/config"
)

func TestExternalProviderAdapter_Doctor(t *testing.T) {
	exe := buildStubPlugin(t)
	p := NewExternalProviderAdapter("stub", exe)

	cfg := &config.Config{Service: "svc"}
	res, err := p.Doctor(context.Background(), providers.DoctorRequest{Config: (*providers.Config)(cfg), Stage: "dev"})
	if err != nil {
		t.Fatalf("Doctor error: %v", err)
	}
	if res.Provider == "" {
		t.Fatalf("expected provider set")
	}
	if len(res.Checks) == 0 {
		t.Fatalf("expected checks")
	}
}

func buildStubPlugin(t *testing.T) string {
	t.Helper()
	src := filepath.Join("testdata", "stubplugin")
	out, err := filepath.Abs(filepath.Join(src, "stubplugin.testbin"))
	if err != nil {
		t.Fatalf("resolve plugin output path: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(out) })
	cmd := exec.Command("go", "build", "-o", out, ".")
	cmd.Dir = src
	cmd.Env = os.Environ()
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build stub plugin: %v\n%s", err, string(b))
	}
	_ = os.Chmod(out, 0o755)
	return out
}

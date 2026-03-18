package external

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/extensions/providers"
)

func TestExternalProviderAdapter_Doctor(t *testing.T) {
	exe := buildStubPlugin(t)
	p := NewExternalProviderAdapter("stub", exe)

	cfg := &config.Config{Service: "svc"}
	res, err := p.Doctor((*providers.Config)(cfg), "dev")
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
	tmp := t.TempDir()
	out := filepath.Join(tmp, "stubplugin")
	src := filepath.Join("testdata", "stubplugin")
	cmd := exec.Command("go", "build", "-o", out, ".")
	cmd.Dir = src
	cmd.Env = os.Environ()
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build stub plugin: %v\n%s", err, string(b))
	}
	return out
}

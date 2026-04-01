package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestBootstrap_ResolvesSecretManagerRefsViaSecretManagerPlugin(t *testing.T) {
	home := t.TempDir()
	t.Setenv("RUNFABRIC_HOME", home)

	pluginDir := filepath.Join(home, "plugins", "secret-managers", "stub-secret-manager", "1.0.0")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("mkdir plugin dir: %v", err)
	}
	executable := filepath.Join(pluginDir, "stubplugin")
	buildStubSecretManagerBinary(t, executable)

	pluginYAML := []byte(`apiVersion: runfabric.io/plugin/v1
kind: secret-manager
id: stub-secret-manager
name: Stub Secret Manager
version: 1.0.0
executable: stubplugin
capabilities:
  - resolve-secret
  - scheme:vault
`)
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), pluginYAML, 0o644); err != nil {
		t.Fatalf("write plugin.yaml: %v", err)
	}

	project := t.TempDir()
	configPath := filepath.Join(project, "runfabric.yml")
	providerName, runtimeName := testProviderNameAndRuntime(t)
	configYAML := []byte(fmt.Sprintf(
		"service: svc\n"+
			"provider:\n"+
			"  name: %s\n"+
			"  runtime: %s\n"+
			"extensions:\n"+
			"  secretManagerPlugin: stub-secret-manager\n"+
			"secrets:\n"+
			"  db_password: vault://apps/team/prod/db-password\n"+
			"functions:\n"+
			"  - name: api\n"+
			"    entry: src/handler.default\n"+
			"    env:\n"+
			"      DB_PASSWORD: \"${secret:db_password}\"\n",
		providerName,
		runtimeName,
	))
	if err := os.WriteFile(configPath, configYAML, 0o644); err != nil {
		t.Fatalf("write runfabric.yml: %v", err)
	}

	ctx, err := Bootstrap(configPath, "prod", "")
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	got := ctx.Config.Functions["api"].Environment["DB_PASSWORD"]
	want := "resolved:vault://apps/team/prod/db-password"
	if got != want {
		t.Fatalf("DB_PASSWORD=%q want %q", got, want)
	}
}

func buildStubSecretManagerBinary(t *testing.T, output string) {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve caller path")
	}
	stubSource := filepath.Join(
		filepath.Dir(file),
		"..",
		"..",
		"extensions",
		"application",
		"external",
		"testdata",
		"stubplugin",
	)
	cmd := exec.Command("go", "build", "-o", output, ".")
	cmd.Dir = stubSource
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build stub secret manager binary: %v\n%s", err, string(b))
	}
	if err := os.Chmod(output, 0o755); err != nil {
		t.Fatalf("chmod stub secret manager binary: %v", err)
	}
}

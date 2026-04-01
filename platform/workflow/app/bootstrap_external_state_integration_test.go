//go:build integration
// +build integration

package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestBootstrap_UsesInstalledExternalStatePlugin_EndToEnd(t *testing.T) {
	home := t.TempDir()
	t.Setenv("RUNFABRIC_HOME", home)

	pluginDir := filepath.Join(home, "plugins", "states", "custom-state", "1.0.0")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("mkdir plugin dir: %v", err)
	}
	executable := filepath.Join(pluginDir, "stubplugin")
	buildStubStatePluginBinary(t, executable)

	pluginYAML := []byte(`apiVersion: runfabric.io/plugin/v1
kind: state
id: custom-state
name: Custom State
version: 1.0.0
capabilities:
  - backend:custom
executable: stubplugin
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
			"  statePlugin: custom-state\n"+
			"  statePluginVersion: 1.0.0\n"+
			"functions:\n"+
			"  - name: api\n"+
			"    entry: src/handler.default\n",
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
	if got := ctx.Backends.Receipts.Kind(); got != "custom" {
		t.Fatalf("receipt backend kind=%q want custom", got)
	}
	if got := ctx.Backends.Locks.Kind(); got != "custom" {
		t.Fatalf("lock backend kind=%q want custom", got)
	}

	entries, err := ctx.Backends.Receipts.ListReleases()
	if err != nil {
		t.Fatalf("list releases: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one release from external state plugin")
	}
}

func buildStubStatePluginBinary(t *testing.T, output string) {
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
		t.Fatalf("build stub state plugin binary: %v\n%s", err, string(b))
	}
	if err := os.Chmod(output, 0o755); err != nil {
		t.Fatalf("chmod stub state plugin binary: %v", err)
	}
}

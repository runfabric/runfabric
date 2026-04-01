package resolution

import (
	"os"
	"path/filepath"
	"testing"
)

func writeExternalProvider(t *testing.T, home, id, version string) {
	t.Helper()
	base := filepath.Join(home, "plugins", "providers", id, version)
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatalf("mkdir plugin dir: %v", err)
	}
	execPath := filepath.Join(base, "plugin")
	if err := os.WriteFile(execPath, []byte("#!/bin/sh\necho ok\n"), 0o755); err != nil {
		t.Fatalf("write plugin executable: %v", err)
	}
	manifest := "apiVersion: runfabric.io/plugin/v1\n" +
		"kind: provider\n" +
		"id: " + id + "\n" +
		"name: " + id + "\n" +
		"version: " + version + "\n" +
		"capabilities:\n" +
		"  - deploy\n" +
		"supportsRuntime:\n" +
		"  - nodejs\n" +
		"supportsTriggers:\n" +
		"  - http\n" +
		"executable: ./plugin\n"
	if err := os.WriteFile(filepath.Join(base, "plugin.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write plugin manifest: %v", err)
	}
}

func writeExternalRouter(t *testing.T, home, id, version string) {
	t.Helper()
	base := filepath.Join(home, "plugins", "routers", id, version)
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatalf("mkdir router plugin dir: %v", err)
	}
	execPath := filepath.Join(base, "plugin")
	if err := os.WriteFile(execPath, []byte("#!/bin/sh\necho ok\n"), 0o755); err != nil {
		t.Fatalf("write router plugin executable: %v", err)
	}
	manifest := "apiVersion: runfabric.io/plugin/v1\n" +
		"kind: router\n" +
		"id: " + id + "\n" +
		"name: " + id + "\n" +
		"version: " + version + "\n" +
		"description: external router plugin\n" +
		"executable: ./plugin\n"
	if err := os.WriteFile(filepath.Join(base, "plugin.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write router plugin manifest: %v", err)
	}
}

func writeExternalRuntime(t *testing.T, home, id, version string) {
	t.Helper()
	base := filepath.Join(home, "plugins", "runtimes", id, version)
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatalf("mkdir runtime plugin dir: %v", err)
	}
	execPath := filepath.Join(base, "plugin")
	if err := os.WriteFile(execPath, []byte("#!/bin/sh\necho ok\n"), 0o755); err != nil {
		t.Fatalf("write runtime plugin executable: %v", err)
	}
	manifest := "apiVersion: runfabric.io/plugin/v1\n" +
		"kind: runtime\n" +
		"id: " + id + "\n" +
		"name: " + id + "\n" +
		"version: " + version + "\n" +
		"description: external runtime plugin\n" +
		"executable: ./plugin\n"
	if err := os.WriteFile(filepath.Join(base, "plugin.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write runtime plugin manifest: %v", err)
	}
}

func writeExternalSimulator(t *testing.T, home, id, version string) {
	t.Helper()
	base := filepath.Join(home, "plugins", "simulators", id, version)
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatalf("mkdir simulator plugin dir: %v", err)
	}
	execPath := filepath.Join(base, "plugin")
	if err := os.WriteFile(execPath, []byte("#!/bin/sh\necho ok\n"), 0o755); err != nil {
		t.Fatalf("write simulator plugin executable: %v", err)
	}
	manifest := "apiVersion: runfabric.io/plugin/v1\n" +
		"kind: simulator\n" +
		"id: " + id + "\n" +
		"name: " + id + "\n" +
		"version: " + version + "\n" +
		"description: external simulator plugin\n" +
		"executable: ./plugin\n"
	if err := os.WriteFile(filepath.Join(base, "plugin.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write simulator plugin manifest: %v", err)
	}
}

func writeExternalSecretManager(t *testing.T, home, id, version string) {
	t.Helper()
	base := filepath.Join(home, "plugins", "secret-managers", id, version)
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatalf("mkdir secret-manager plugin dir: %v", err)
	}
	execPath := filepath.Join(base, "plugin")
	if err := os.WriteFile(execPath, []byte("#!/bin/sh\necho ok\n"), 0o755); err != nil {
		t.Fatalf("write secret-manager plugin executable: %v", err)
	}
	manifest := "apiVersion: runfabric.io/plugin/v1\n" +
		"kind: secret-manager\n" +
		"id: " + id + "\n" +
		"name: " + id + "\n" +
		"version: " + version + "\n" +
		"description: external secret-manager plugin\n" +
		"capabilities:\n" +
		"  - scheme:stub-sm\n" +
		"executable: ./plugin\n"
	if err := os.WriteFile(filepath.Join(base, "plugin.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write secret-manager plugin manifest: %v", err)
	}
}

func writeExternalState(t *testing.T, home, id, version, backendKind string) {
	t.Helper()
	base := filepath.Join(home, "plugins", "states", id, version)
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatalf("mkdir state plugin dir: %v", err)
	}
	execPath := filepath.Join(base, "plugin")
	if err := os.WriteFile(execPath, []byte("#!/bin/sh\necho ok\n"), 0o755); err != nil {
		t.Fatalf("write state plugin executable: %v", err)
	}
	manifest := "apiVersion: runfabric.io/plugin/v1\n" +
		"kind: state\n" +
		"id: " + id + "\n" +
		"name: " + id + "\n" +
		"version: " + version + "\n" +
		"description: external state plugin\n" +
		"capabilities:\n" +
		"  - backend:" + backendKind + "\n" +
		"executable: ./plugin\n"
	if err := os.WriteFile(filepath.Join(base, "plugin.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write state plugin manifest: %v", err)
	}
}

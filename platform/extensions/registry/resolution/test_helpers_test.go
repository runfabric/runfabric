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
	manifest := "apiVersion: runfabric.dev/v1\n" +
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
	manifest := "apiVersion: runfabric.dev/v1\n" +
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

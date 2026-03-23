package vercel_test

import (
	"os"
	"path/filepath"
	"testing"

	legacyresolution "github.com/runfabric/runfabric/platform/extension/resolution"
	registryresolution "github.com/runfabric/runfabric/platform/extensions/registry/resolution"
)

func TestVercelResolution_IsResolvedAndAPIDispatch(t *testing.T) {
	legacy, err := legacyresolution.New(legacyresolution.Options{IncludeExternal: false})
	if err != nil {
		t.Fatalf("legacy boundary: %v", err)
	}
	if _, err := legacy.ResolveProvider("vercel"); err != nil {
		t.Fatalf("legacy resolve vercel: %v", err)
	}
	if !legacy.IsAPIDispatchProvider("vercel") {
		t.Fatal("legacy expected vercel to be API-dispatched")
	}

	registry, err := registryresolution.New(registryresolution.Options{IncludeExternal: false})
	if err != nil {
		t.Fatalf("registry boundary: %v", err)
	}
	if _, err := registry.ResolveProvider("vercel"); err != nil {
		t.Fatalf("registry resolve vercel: %v", err)
	}
	if !registry.IsAPIDispatchProvider("vercel") {
		t.Fatal("registry expected vercel to be API-dispatched")
	}
}

func TestVercelResolution_PreferExternalHonorsPinnedVersion(t *testing.T) {
	home := t.TempDir()
	t.Setenv("RUNFABRIC_HOME", home)
	writeExternalProvider(t, home, "vercel", "1.0.0")
	writeExternalProvider(t, home, "vercel", "2.0.0")

	legacy, err := legacyresolution.New(legacyresolution.Options{
		IncludeExternal: true,
		PreferExternal:  true,
		PinnedVersions: map[string]string{
			"vercel": "1.0.0",
		},
	})
	if err != nil {
		t.Fatalf("legacy boundary: %v", err)
	}
	vercelLegacy := legacy.PluginRegistry().Get("vercel")
	if vercelLegacy == nil {
		t.Fatal("legacy expected vercel plugin manifest")
	}
	if vercelLegacy.Source != "external" {
		t.Fatalf("legacy expected external source, got %q", vercelLegacy.Source)
	}
	if vercelLegacy.Version != "1.0.0" {
		t.Fatalf("legacy expected version 1.0.0, got %q", vercelLegacy.Version)
	}

	registry, err := registryresolution.New(registryresolution.Options{
		IncludeExternal: true,
		PreferExternal:  true,
		PinnedVersions: map[string]string{
			"vercel": "1.0.0",
		},
	})
	if err != nil {
		t.Fatalf("registry boundary: %v", err)
	}
	vercelRegistry := registry.PluginRegistry().Get("vercel")
	if vercelRegistry == nil {
		t.Fatal("registry expected vercel plugin manifest")
	}
	if vercelRegistry.Source != "external" {
		t.Fatalf("registry expected external source, got %q", vercelRegistry.Source)
	}
	if vercelRegistry.Version != "1.0.0" {
		t.Fatalf("registry expected version 1.0.0, got %q", vercelRegistry.Version)
	}
}

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

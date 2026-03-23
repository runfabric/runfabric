package aws_test

import (
	"os"
	"path/filepath"
	"testing"

	legacyresolution "github.com/runfabric/runfabric/platform/extension/resolution"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy"
	registryresolution "github.com/runfabric/runfabric/platform/extensions/registry/resolution"
)

func TestBuiltinProviders_ResolveAndNotAPIDispatch(t *testing.T) {
	builtinIDs := providerpolicy.BuiltinImplementationIDs()
	if len(builtinIDs) == 0 {
		t.Fatal("expected at least one builtin provider ID from policy")
	}

	legacy, err := legacyresolution.New(legacyresolution.Options{IncludeExternal: false})
	if err != nil {
		t.Fatalf("legacy boundary: %v", err)
	}
	registry, err := registryresolution.New(registryresolution.Options{IncludeExternal: false})
	if err != nil {
		t.Fatalf("registry boundary: %v", err)
	}

	for _, id := range builtinIDs {
		if _, err := legacy.ResolveProvider(id); err != nil {
			t.Errorf("legacy: expected %q to resolve as built-in: %v", id, err)
		}
		if _, err := registry.ResolveProvider(id); err != nil {
			t.Errorf("registry: expected %q to resolve as built-in: %v", id, err)
		}
	}
}

func TestBuiltinProvider_PreferExternalHonorsPinnedVersion(t *testing.T) {
	builtinIDs := providerpolicy.BuiltinImplementationIDs()
	if len(builtinIDs) == 0 {
		t.Skip("no builtin providers defined in policy")
	}
	id := builtinIDs[0]
	const version = "9.9.9"

	home := t.TempDir()
	t.Setenv("RUNFABRIC_HOME", home)
	writeExternalProvider(t, home, id, version)

	legacy, err := legacyresolution.New(legacyresolution.Options{
		IncludeExternal: true,
		PreferExternal:  true,
		PinnedVersions:  map[string]string{id: version},
	})
	if err != nil {
		t.Fatalf("legacy boundary: %v", err)
	}
	if p := legacy.PluginRegistry().Get(id); p == nil {
		t.Fatalf("legacy: expected %q plugin manifest", id)
	} else if p.Source != "external" {
		t.Fatalf("legacy: expected external source, got %q", p.Source)
	} else if p.Version != version {
		t.Fatalf("legacy: expected version %s, got %q", version, p.Version)
	}

	registry, err := registryresolution.New(registryresolution.Options{
		IncludeExternal: true,
		PreferExternal:  true,
		PinnedVersions:  map[string]string{id: version},
	})
	if err != nil {
		t.Fatalf("registry boundary: %v", err)
	}
	if p := registry.PluginRegistry().Get(id); p == nil {
		t.Fatalf("registry: expected %q plugin manifest", id)
	} else if p.Source != "external" {
		t.Fatalf("registry: expected external source, got %q", p.Source)
	} else if p.Version != version {
		t.Fatalf("registry: expected version %s, got %q", version, p.Version)
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

package resolution

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewBoundary_RegistersBuiltinAndAPIProviders(t *testing.T) {
	b, err := New(Options{IncludeExternal: false})
	if err != nil {
		t.Fatalf("new boundary: %v", err)
	}

	for _, name := range []string{"aws-lambda", "gcp-functions", "vercel", "netlify"} {
		if _, err := b.ResolveProvider(name); err != nil {
			t.Fatalf("resolve provider %q: %v", name, err)
		}
	}
}

func TestResolveRuntime_NormalizesVersionedRuntimeIDs(t *testing.T) {
	b, err := New(Options{IncludeExternal: false})
	if err != nil {
		t.Fatalf("new boundary: %v", err)
	}

	got, err := b.ResolveRuntime("nodejs20.x")
	if err != nil {
		t.Fatalf("resolve nodejs20.x: %v", err)
	}
	if got.ID != "nodejs" {
		t.Fatalf("runtime id = %q, want nodejs", got.ID)
	}

	got, err = b.ResolveRuntime("python3.11")
	if err != nil {
		t.Fatalf("resolve python3.11: %v", err)
	}
	if got.ID != "python" {
		t.Fatalf("runtime id = %q, want python", got.ID)
	}

	rt, err := b.ResolveRuntimePlugin("nodejs20.x")
	if err != nil {
		t.Fatalf("resolve runtime plugin nodejs20.x: %v", err)
	}
	if rt.Meta().ID != "nodejs" {
		t.Fatalf("runtime plugin id = %q, want nodejs", rt.Meta().ID)
	}
}

func TestResolveRuntime_UnknownRuntimeErrors(t *testing.T) {
	b, err := New(Options{IncludeExternal: false})
	if err != nil {
		t.Fatalf("new boundary: %v", err)
	}

	if _, err := b.ResolveRuntime("unknown-runtime"); err == nil {
		t.Fatal("expected unknown runtime to return error")
	}
}

func TestIsInternalProvider_AWSOnly(t *testing.T) {
	b, err := New(Options{IncludeExternal: false})
	if err != nil {
		t.Fatalf("new boundary: %v", err)
	}

	if !b.IsInternalProvider("aws-lambda") {
		t.Fatal("expected aws-lambda to be an internal provider")
	}
	if b.IsInternalProvider("aws") {
		t.Fatal("expected aws alias to be removed from internal providers")
	}
	if b.IsInternalProvider("vercel") {
		t.Fatal("expected vercel to be non-internal")
	}
}

func TestIsAPIDispatchProvider(t *testing.T) {
	b, err := New(Options{IncludeExternal: false})
	if err != nil {
		t.Fatalf("new boundary: %v", err)
	}

	if !b.IsAPIDispatchProvider("gcp-functions") {
		t.Fatal("expected gcp-functions to be API-dispatched")
	}
	if !b.IsAPIDispatchProvider("vercel") {
		t.Fatal("expected vercel to be API-dispatched")
	}
	if b.IsAPIDispatchProvider("aws-lambda") {
		t.Fatal("expected aws-lambda to be non-API-dispatched")
	}
}

func TestResolveSimulator_BuiltinLocal(t *testing.T) {
	b, err := New(Options{IncludeExternal: false})
	if err != nil {
		t.Fatalf("new boundary: %v", err)
	}
	sim, err := b.ResolveSimulator("local")
	if err != nil {
		t.Fatalf("resolve local simulator: %v", err)
	}
	if sim.Meta().ID != "local" {
		t.Fatalf("simulator id = %q, want local", sim.Meta().ID)
	}
}

func TestNewBoundary_PreferExternalHonorsVersionPinAndKeepsAWSInternal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("RUNFABRIC_HOME", home)

	writeExternalProvider(t, home, "vercel", "1.0.0")
	writeExternalProvider(t, home, "vercel", "2.0.0")
	writeExternalProvider(t, home, "aws-lambda", "9.9.9")

	b, err := New(Options{
		IncludeExternal: true,
		PreferExternal:  true,
		PinnedVersions: map[string]string{
			"vercel":     "1.0.0",
			"aws-lambda": "9.9.9",
		},
	})
	if err != nil {
		t.Fatalf("new boundary: %v", err)
	}

	vercel := b.PluginRegistry().Get("vercel")
	if vercel == nil {
		t.Fatal("expected vercel plugin manifest")
	}
	if vercel.Source != "external" {
		t.Fatalf("expected external vercel manifest, got source=%q", vercel.Source)
	}
	if vercel.Version != "1.0.0" {
		t.Fatalf("expected pinned vercel version 1.0.0, got %q", vercel.Version)
	}

	aws := b.PluginRegistry().Get("aws-lambda")
	if aws == nil {
		t.Fatal("expected aws-lambda plugin manifest")
	}
	if aws.Source == "external" {
		t.Fatal("aws-lambda must remain internal even when preferExternal=true")
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
		"executable: ./plugin\n"
	if err := os.WriteFile(filepath.Join(base, "plugin.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write plugin manifest: %v", err)
	}
}

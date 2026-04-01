package app

import (
	"testing"

	"github.com/runfabric/runfabric/platform/core/model/config"
	manifests "github.com/runfabric/runfabric/platform/extensions/manifest"
	"github.com/runfabric/runfabric/platform/extensions/registry/resolution"
)

func TestResolveStateBackendKind_BuiltinLocalState(t *testing.T) {
	cfg := &config.Config{
		Extensions: map[string]any{
			"statePlugin": "local",
		},
	}
	b, err := resolution.New(resolution.Options{IncludeExternal: false})
	if err != nil {
		t.Fatalf("new boundary: %v", err)
	}

	kind, err := resolveStateBackendKind(cfg, b)
	if err != nil {
		t.Fatalf("resolveStateBackendKind: %v", err)
	}
	if kind != "local" {
		t.Fatalf("kind=%q want local", kind)
	}
}

func TestResolveStateBackendKind_BuiltinLocalAlias(t *testing.T) {
	cfg := &config.Config{
		Extensions: map[string]any{
			"statePlugin": "Local State Backend",
		},
	}
	b, err := resolution.New(resolution.Options{IncludeExternal: false})
	if err != nil {
		t.Fatalf("new boundary: %v", err)
	}
	b.PluginRegistry().Register(&manifests.PluginManifest{
		ID:     "local",
		Name:   "Local State Backend",
		Kind:   manifests.KindState,
		Source: "builtin",
	})

	kind, err := resolveStateBackendKind(cfg, b)
	if err != nil {
		t.Fatalf("resolveStateBackendKind local alias: %v", err)
	}
	if kind != "local" {
		t.Fatalf("kind=%q want local", kind)
	}
}

func TestResolveStateBackendKind_RegisteredUsesDerivedToken(t *testing.T) {
	cfg := &config.Config{
		Extensions: map[string]any{
			"statePlugin": "custom",
		},
	}
	b, err := resolution.New(resolution.Options{IncludeExternal: false})
	if err != nil {
		t.Fatalf("new boundary: %v", err)
	}
	b.PluginRegistry().Register(&manifests.PluginManifest{ID: "custom", Kind: manifests.KindState, Source: "builtin"})

	kind, err := resolveStateBackendKind(cfg, b)
	if err != nil {
		t.Fatalf("resolveStateBackendKind custom: %v", err)
	}
	if kind != "custom" {
		t.Fatalf("kind=%q want custom", kind)
	}
}

func TestResolveStateBackendKind_ParsesProviderPrefixedStateID(t *testing.T) {
	cfg := &config.Config{
		Extensions: map[string]any{
			"statePlugin": "aws-dynamodb",
		},
	}
	b, err := resolution.New(resolution.Options{IncludeExternal: false})
	if err != nil {
		t.Fatalf("new boundary: %v", err)
	}
	b.PluginRegistry().Register(&manifests.PluginManifest{ID: "aws-dynamodb", Kind: manifests.KindState, Source: "builtin"})

	kind, err := resolveStateBackendKind(cfg, b)
	if err != nil {
		t.Fatalf("resolveStateBackendKind provider-prefixed id: %v", err)
	}
	if kind != "dynamodb" {
		t.Fatalf("kind=%q want dynamodb", kind)
	}
}

func TestNormalizePluginIDByKind_SecretManagerByName(t *testing.T) {
	reg := manifests.NewEmptyPluginRegistry()
	reg.Register(&manifests.PluginManifest{
		ID:   "vault-secret-manager",
		Name: "Vault Secret Manager",
		Kind: manifests.KindSecretManager,
	})
	id, err := normalizePluginIDByKind(manifests.KindSecretManager, "Vault Secret Manager", reg)
	if err != nil {
		t.Fatalf("normalizePluginIDByKind secret-manager by name: %v", err)
	}
	if id != "vault-secret-manager" {
		t.Fatalf("id=%q want vault-secret-manager", id)
	}
}

func TestNormalizePluginIDByKind_ByNameAndKind(t *testing.T) {
	reg := manifests.NewEmptyPluginRegistry()
	reg.Register(&manifests.PluginManifest{ID: "edge-router", Name: "Edge Router", Kind: manifests.KindRouter})
	reg.Register(&manifests.PluginManifest{ID: "nodejs", Name: "Node.js Runtime", Kind: manifests.KindRuntime})

	routerID, err := normalizePluginIDByKind(manifests.KindRouter, "Edge Router", reg)
	if err != nil {
		t.Fatalf("normalize router by name: %v", err)
	}
	if routerID != "edge-router" {
		t.Fatalf("router id=%q want edge-router", routerID)
	}

	runtimeID, err := normalizePluginIDByKind(manifests.KindRuntime, "Node.js Runtime", reg)
	if err != nil {
		t.Fatalf("normalize runtime by name: %v", err)
	}
	if runtimeID != "nodejs" {
		t.Fatalf("runtime id=%q want nodejs", runtimeID)
	}
}

func TestBackendOptionsFromConfigAndEnv_StatePluginOverride(t *testing.T) {
	cfg := &config.Config{
		Backend: &config.BackendConfig{Kind: "postgres"},
	}
	opts := backendOptionsFromConfigAndEnv(cfg, t.TempDir(), "local")
	if opts.Kind != "local" {
		t.Fatalf("opts.Kind=%q want local", opts.Kind)
	}
}

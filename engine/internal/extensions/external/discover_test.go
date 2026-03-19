package external

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDiscoverLatest_SelectsLatestVersionPerID(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(envHome, tmp)

	// providers/foo/{0.1.0,0.2.0}
	writePlugin(t, filepath.Join(tmp, "plugins", "providers", "foo", "0.1.0"), pluginYAML{
		APIVersion:  "runfabric.io/plugin/v1",
		Kind:        "provider",
		ID:          "foo",
		Name:        "Foo",
		Description: "Foo provider",
		Version:     "0.1.0",
		Executable:  "runfabric-provider-foo",
	})
	writePlugin(t, filepath.Join(tmp, "plugins", "providers", "foo", "0.2.0"), pluginYAML{
		APIVersion:  "runfabric.io/plugin/v1",
		Kind:        "provider",
		ID:          "foo",
		Name:        "Foo",
		Description: "Foo provider",
		Version:     "0.2.0",
		Executable:  "runfabric-provider-foo",
	})

	plugins, err := DiscoverLatest()
	if err != nil {
		t.Fatalf("DiscoverLatest error: %v", err)
	}
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}
	if plugins[0].ID != "foo" {
		t.Fatalf("expected id foo, got %s", plugins[0].ID)
	}
	if plugins[0].Version != "0.2.0" {
		t.Fatalf("expected version 0.2.0, got %s", plugins[0].Version)
	}
	if plugins[0].Source != "external" {
		t.Fatalf("expected source external, got %s", plugins[0].Source)
	}
	if plugins[0].Path == "" {
		t.Fatal("expected path to be set")
	}
	if plugins[0].Executable == "" {
		t.Fatal("expected executable to be set")
	}
}

func TestDiscoverLatest_SupportsKindAliasesAndDirectoryAliases(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(envHome, tmp)

	writePlugin(t, filepath.Join(tmp, "plugins", "provider", "alias-provider", "1.2.3"), pluginYAML{
		APIVersion: "runfabric.io/plugin/v1",
		Kind:       "providers",
		ID:         "alias-provider",
		Name:       "Alias Provider",
		Version:    "1.2.3",
		Executable: "runfabric-provider-alias",
	})

	plugins, err := DiscoverLatest()
	if err != nil {
		t.Fatalf("DiscoverLatest error: %v", err)
	}
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}
	if plugins[0].Kind != "provider" {
		t.Fatalf("expected normalized kind provider, got %q", plugins[0].Kind)
	}
	if plugins[0].ID != "alias-provider" {
		t.Fatalf("expected alias-provider, got %q", plugins[0].ID)
	}
}

func writePlugin(t *testing.T, dir string, m pluginYAML) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Create dummy executable target.
	if m.Executable != "" {
		execPath := filepath.Join(dir, m.Executable)
		if err := os.WriteFile(execPath, []byte("x"), 0o755); err != nil {
			t.Fatalf("write exec: %v", err)
		}
	}
	data, err := yaml.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.yaml"), data, 0o644); err != nil {
		t.Fatalf("write plugin.yaml: %v", err)
	}
}

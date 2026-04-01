package external

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDiscoverLatest_SelectsLatestVersionPerID(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(envHome, tmp)

	// providers/foo/{0.1.0,0.2.0}
	writePlugin(t, filepath.Join(tmp, "plugins", "providers", "foo", "0.1.0"), pluginYAML{
		APIVersion:       "runfabric.io/plugin/v1",
		Kind:             "provider",
		ID:               "foo",
		Name:             "Foo",
		Description:      "Foo provider",
		Version:          "0.1.0",
		Executable:       "runfabric-provider-foo",
		Capabilities:     []string{"deploy"},
		SupportsTriggers: []string{"http"},
		SupportsRuntime:  []string{"nodejs"},
	})
	writePlugin(t, filepath.Join(tmp, "plugins", "providers", "foo", "0.2.0"), pluginYAML{
		APIVersion:       "runfabric.io/plugin/v1",
		Kind:             "provider",
		ID:               "foo",
		Name:             "Foo",
		Description:      "Foo provider",
		Version:          "0.2.0",
		Executable:       "runfabric-provider-foo",
		Capabilities:     []string{"deploy", "doctor"},
		SupportsTriggers: []string{"http", "cron"},
		SupportsRuntime:  []string{"nodejs"},
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
	if len(plugins[0].Capabilities) != 2 || plugins[0].Capabilities[0] != "deploy" {
		t.Fatalf("expected hydrated capabilities, got %#v", plugins[0].Capabilities)
	}
	if len(plugins[0].SupportsTriggers) != 2 {
		t.Fatalf("expected hydrated triggers, got %#v", plugins[0].SupportsTriggers)
	}
}

func TestDiscoverLatest_SecretManagerKind(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(envHome, tmp)

	writePlugin(t, filepath.Join(tmp, "plugins", "secret-managers", "vault-sm", "1.0.0"), pluginYAML{
		APIVersion:  "runfabric.io/plugin/v1",
		Kind:        "secret-manager",
		ID:          "vault-sm",
		Name:        "Vault Secret Manager",
		Description: "Vault-backed secret manager",
		Version:     "1.0.0",
		Executable:  "runfabric-secret-manager-vault",
	})

	plugins, err := DiscoverLatest()
	if err != nil {
		t.Fatalf("DiscoverLatest error: %v", err)
	}
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}
	if string(plugins[0].Kind) != "secret-manager" {
		t.Fatalf("expected secret-manager kind, got %q", plugins[0].Kind)
	}
	if plugins[0].ID != "vault-sm" {
		t.Fatalf("expected id vault-sm, got %q", plugins[0].ID)
	}
}

func TestDiscoverLatest_StateKind(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(envHome, tmp)

	writePlugin(t, filepath.Join(tmp, "plugins", "states", "local", "1.0.0"), pluginYAML{
		APIVersion:  "runfabric.io/plugin/v1",
		Kind:        "state",
		ID:          "local",
		Name:        "Local State Backend",
		Description: "Uses local file backend for state",
		Version:     "1.0.0",
		Executable:  "runfabric-state-local",
	})

	plugins, err := DiscoverLatest()
	if err != nil {
		t.Fatalf("DiscoverLatest error: %v", err)
	}
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}
	if string(plugins[0].Kind) != "state" {
		t.Fatalf("expected state kind, got %q", plugins[0].Kind)
	}
	if plugins[0].ID != "local" {
		t.Fatalf("expected id local, got %q", plugins[0].ID)
	}
}

func TestDiscoverLatest_RejectsPluralPluginKind(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(envHome, tmp)

	writePlugin(t, filepath.Join(tmp, "plugins", "provider", "alias-provider", "1.2.3"), pluginYAML{
		APIVersion:       "runfabric.io/plugin/v1",
		Kind:             "providers",
		ID:               "alias-provider",
		Name:             "Alias Provider",
		Version:          "1.2.3",
		Executable:       "runfabric-provider-alias",
		Capabilities:     []string{"deploy"},
		SupportsTriggers: []string{"http"},
		SupportsRuntime:  []string{"nodejs"},
	})

	plugins, err := DiscoverLatest()
	if err != nil {
		t.Fatalf("DiscoverLatest error: %v", err)
	}
	if len(plugins) != 0 {
		t.Fatalf("expected plural plugin kind to be rejected, got %d plugin(s)", len(plugins))
	}
}

func TestDiscover_ProviderMissingCapabilitiesMarkedInvalid(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(envHome, tmp)

	writePlugin(t, filepath.Join(tmp, "plugins", "providers", "broken-caps", "0.1.0"), pluginYAML{
		APIVersion:       "runfabric.io/plugin/v1",
		Kind:             "provider",
		ID:               "broken-caps",
		Name:             "Broken Provider",
		Version:          "0.1.0",
		Executable:       "runfabric-provider-broken",
		SupportsTriggers: []string{"http"},
		SupportsRuntime:  []string{"nodejs"},
	})

	res, err := Discover(DiscoverOptions{IncludeInvalid: true})
	if err != nil {
		t.Fatalf("Discover error: %v", err)
	}
	if len(res.Plugins) != 0 {
		t.Fatalf("expected invalid provider to be skipped, got %#v", res.Plugins)
	}
	if len(res.Invalid) == 0 || !strings.Contains(res.Invalid[0].Reason, "must declare capabilities") {
		t.Fatalf("expected capabilities validation error, got %#v", res.Invalid)
	}
}

func TestDiscover_ProviderMissingSupportsTriggers(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(envHome, tmp)

	writePlugin(t, filepath.Join(tmp, "plugins", "providers", "broken-triggers", "0.1.0"), pluginYAML{
		APIVersion:      "runfabric.io/plugin/v1",
		Kind:            "provider",
		ID:              "broken-triggers",
		Name:            "Broken Provider",
		Version:         "0.1.0",
		Executable:      "runfabric-provider-broken",
		Capabilities:    []string{"http"},
		SupportsRuntime: []string{"nodejs"},
	})

	res, err := Discover(DiscoverOptions{IncludeInvalid: true})
	if err != nil {
		t.Fatalf("Discover error: %v", err)
	}
	if len(res.Plugins) != 0 {
		t.Fatalf("expected invalid provider to be skipped, got %#v", res.Plugins)
	}
	if len(res.Invalid) == 0 || !strings.Contains(res.Invalid[0].Reason, "must declare supportsTriggers") {
		t.Fatalf("expected supportsTriggers validation error, got %#v", res.Invalid)
	}
}

func TestDiscover_ProviderMissingSupportsRuntime(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(envHome, tmp)

	writePlugin(t, filepath.Join(tmp, "plugins", "providers", "broken-runtime", "0.1.0"), pluginYAML{
		APIVersion:       "runfabric.io/plugin/v1",
		Kind:             "provider",
		ID:               "broken-runtime",
		Name:             "Broken Provider",
		Version:          "0.1.0",
		Executable:       "runfabric-provider-broken",
		Capabilities:     []string{"http"},
		SupportsTriggers: []string{"http"},
	})

	res, err := Discover(DiscoverOptions{IncludeInvalid: true})
	if err != nil {
		t.Fatalf("Discover error: %v", err)
	}
	if len(res.Plugins) != 0 {
		t.Fatalf("expected invalid provider to be skipped, got %#v", res.Plugins)
	}
	if len(res.Invalid) == 0 || !strings.Contains(res.Invalid[0].Reason, "must declare supportsRuntime") {
		t.Fatalf("expected supportsRuntime validation error, got %#v", res.Invalid)
	}
}

func TestDiscover_RejectsUnsupportedAPIVersion(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(envHome, tmp)

	writePlugin(t, filepath.Join(tmp, "plugins", "providers", "bad-api-version", "0.1.0"), pluginYAML{
		APIVersion:       "runfabric.io/v1alpha1",
		Kind:             "provider",
		ID:               "bad-api-version",
		Name:             "Bad API Version",
		Version:          "0.1.0",
		Executable:       "runfabric-provider-bad",
		Capabilities:     []string{"deploy"},
		SupportsTriggers: []string{"http"},
		SupportsRuntime:  []string{"nodejs"},
	})

	res, err := Discover(DiscoverOptions{IncludeInvalid: true})
	if err != nil {
		t.Fatalf("Discover error: %v", err)
	}
	if len(res.Plugins) != 0 {
		t.Fatalf("expected invalid plugin to be skipped, got %#v", res.Plugins)
	}
	if len(res.Invalid) == 0 || !strings.Contains(res.Invalid[0].Reason, "unsupported plugin apiVersion") {
		t.Fatalf("expected unsupported apiVersion error, got %#v", res.Invalid)
	}
}

func TestDiscover_RejectsVPrefixedVersionDirectory(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(envHome, tmp)

	writePlugin(t, filepath.Join(tmp, "plugins", "providers", "vprefix", "v1.2.3"), pluginYAML{
		APIVersion:       "runfabric.io/plugin/v1",
		Kind:             "provider",
		ID:               "vprefix",
		Name:             "VPrefixed",
		Version:          "v1.2.3",
		Executable:       "runfabric-provider-vprefix",
		Capabilities:     []string{"deploy"},
		SupportsTriggers: []string{"http"},
		SupportsRuntime:  []string{"nodejs"},
	})

	plugins, err := DiscoverLatest()
	if err != nil {
		t.Fatalf("DiscoverLatest error: %v", err)
	}
	if len(plugins) != 0 {
		t.Fatalf("expected v-prefixed version directory to be ignored, got %d plugin(s)", len(plugins))
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

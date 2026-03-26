package manifests

import (
	"testing"

	"github.com/runfabric/runfabric/platform/extensions/providerpolicy"
)

func TestPluginRegistry_List(t *testing.T) {
	reg := NewPluginRegistry()
	all := reg.List("")
	if len(all) < 2 {
		t.Errorf("expected at least 2 built-in provider plugins, got %d", len(all))
	}
	providers := reg.List(KindProvider)
	if len(providers) < 2 {
		t.Errorf("expected at least 2 provider plugins, got %d", len(providers))
	}
	runtimes := reg.List(KindRuntime)
	if len(runtimes) != 0 {
		t.Errorf("expected 0 runtime plugins in base manifest registry, got %d", len(runtimes))
	}
}

func TestPluginRegistry_Get(t *testing.T) {
	reg := NewPluginRegistry()
	if reg.Get("aws") != nil {
		t.Fatal("Get(aws) expected nil after alias removal")
	}
	for _, d := range providerpolicy.BuiltinManifestProviders() {
		m := reg.Get(d.ID)
		if m == nil {
			t.Errorf("Get(%q) expected non-nil (defined in BuiltinManifestProviders)", d.ID)
			continue
		}
		if m.Kind != KindProvider {
			t.Errorf("Get(%q) kind: got %s, want %s", d.ID, m.Kind, KindProvider)
		}
	}
	if reg.Get("nonexistent") != nil {
		t.Error("Get(nonexistent) expected nil")
	}
}

func TestPluginRegistry_Search(t *testing.T) {
	reg := NewPluginRegistry()
	empty := reg.Search("")
	if len(empty) == 0 {
		t.Error("Search(\"\") should return all plugins")
	}
	manifestProviders := providerpolicy.BuiltinManifestProviders()
	if len(manifestProviders) > 0 {
		term := manifestProviders[0].ID
		found := reg.Search(term)
		if len(found) < 1 {
			t.Errorf("Search(%q) should return at least one plugin (first builtin manifest provider)", term)
		}
	}
	runtimeTerm := reg.Search("runtime-node")
	if len(runtimeTerm) != 0 {
		t.Error("Search(runtime-node) should be empty in base provider-only registry")
	}
	none := reg.Search("xyznonexistent123")
	if len(none) != 0 {
		t.Errorf("Search(nonexistent) expected 0, got %d", len(none))
	}
}

func TestNormalizePluginKind(t *testing.T) {
	tests := []struct {
		in   string
		want PluginKind
	}{
		{in: "provider", want: KindProvider},
		{in: "providers", want: KindProvider},
		{in: "runtime", want: KindRuntime},
		{in: "runtimes", want: KindRuntime},
		{in: "simulator", want: KindSimulator},
		{in: "simulators", want: KindSimulator},
		{in: "unknown", want: "unknown"},
	}

	for _, tt := range tests {
		got := NormalizePluginKind(tt.in)
		if got != tt.want {
			t.Fatalf("NormalizePluginKind(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestIsSupportedPluginKind(t *testing.T) {
	if !IsSupportedPluginKind(KindProvider) {
		t.Fatal("provider should be supported")
	}
	if !IsSupportedPluginKind(KindRuntime) {
		t.Fatal("runtime should be supported")
	}
	if !IsSupportedPluginKind(KindSimulator) {
		t.Fatal("simulator should be supported")
	}
	if IsSupportedPluginKind("providers") {
		t.Fatal("raw alias should not be supported until normalized")
	}
}

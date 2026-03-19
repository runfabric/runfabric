package manifests

import (
	"testing"
)

func TestPluginRegistry_List(t *testing.T) {
	reg := NewPluginRegistry()
	all := reg.List("")
	if len(all) < 4 {
		t.Errorf("expected at least 4 built-in plugins, got %d", len(all))
	}
	providers := reg.List(KindProvider)
	if len(providers) < 2 {
		t.Errorf("expected at least 2 provider plugins, got %d", len(providers))
	}
	runtimes := reg.List(KindRuntime)
	if len(runtimes) < 2 {
		t.Errorf("expected at least 2 runtime plugins, got %d", len(runtimes))
	}
}

func TestPluginRegistry_Get(t *testing.T) {
	reg := NewPluginRegistry()
	m := reg.Get("aws-lambda")
	if m == nil {
		t.Fatal("Get(aws-lambda) expected non-nil")
	}
	if m.Kind != KindProvider {
		t.Errorf("aws-lambda kind: got %s", m.Kind)
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
	aws := reg.Search("aws")
	if len(aws) < 1 {
		t.Error("Search(aws) should return at least one plugin")
	}
	node := reg.Search("node")
	if len(node) < 1 {
		t.Error("Search(node) should return nodejs or runtime-node")
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

package providers

import "testing"

func TestNewCapabilitySet_NormalizesAndDedupes(t *testing.T) {
	set := NewCapabilitySet(ProviderMeta{Capabilities: []string{" Deploy ", "deploy", "LOGS", ""}})
	if !set.Has("deploy") {
		t.Fatal("expected deploy capability")
	}
	if !set.Has("logs") {
		t.Fatal("expected logs capability")
	}
	if got := len(set.List()); got != 2 {
		t.Fatalf("capability count = %d, want 2", got)
	}
	if !set.HasAny("invoke", "logs") {
		t.Fatal("expected HasAny(invoke,logs) to be true")
	}
}

func TestLoadRegistryAndResolveProvider(t *testing.T) {
	opts := LoadOptions{IncludeExternal: false}
	reg, err := LoadRegistry(opts)
	if err != nil {
		t.Fatalf("load registry: %v", err)
	}
	if reg == nil {
		t.Fatal("expected non-nil registry")
	}
	p, err := ResolveProvider("gcp-functions", opts)
	if err != nil {
		t.Fatalf("resolve provider: %v", err)
	}
	if p == nil {
		t.Fatal("expected provider")
	}
}

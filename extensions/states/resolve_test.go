package states

import "testing"

func TestBackendKindFromPluginID_ProviderPrefixed(t *testing.T) {
	kind, ok := BackendKindFromPluginID("aws-dynamodb")
	if !ok {
		t.Fatalf("expected backend kind from plugin ID")
	}
	if kind != "dynamodb" {
		t.Fatalf("kind=%q want dynamodb", kind)
	}
}

func TestBackendKindFromCapability(t *testing.T) {
	kind, ok := BackendKindFromCapability("backend:postgres")
	if !ok {
		t.Fatalf("expected backend kind from capability")
	}
	if kind != "postgres" {
		t.Fatalf("kind=%q want postgres", kind)
	}
}

func TestBackendKindFromPlugin_PrefersCapability(t *testing.T) {
	kind, ok := BackendKindFromPlugin("custom", []string{"backend:sqlite"})
	if !ok {
		t.Fatalf("expected backend kind from plugin metadata")
	}
	if kind != "sqlite" {
		t.Fatalf("kind=%q want sqlite", kind)
	}
}

func TestNormalizeBackendKindToken_Local(t *testing.T) {
	kind, ok := NormalizeBackendKindToken("local")
	if !ok {
		t.Fatalf("expected normalized backend kind")
	}
	if kind != "local" {
		t.Fatalf("kind=%q want local", kind)
	}
}

func TestExpandLookupAliases_NoOp(t *testing.T) {
	keys := map[string]struct{}{"local": {}}
	ExpandLookupAliases(keys)
	if _, ok := keys["local"]; !ok {
		t.Fatalf("expected local key to remain present")
	}
}

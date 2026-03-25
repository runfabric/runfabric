package gcp

import (
	"testing"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

func TestCloudWorkflowsFromConfigAndBindings(t *testing.T) {
	cfg := sdkprovider.Config{
		"extensions": map[string]any{
			"gcp-functions": map[string]any{
				"cloudWorkflows": []any{
					map[string]any{
						"name":       "order-flow",
						"definition": map[string]any{"main": map[string]any{"steps": []any{}}},
						"bindings":   map[string]any{"createOrder": "createOrder"},
					},
				},
			},
		},
	}

	decls, err := cloudWorkflowsFromConfig(cfg, ".")
	if err != nil {
		t.Fatalf("cloudWorkflowsFromConfig returned error: %v", err)
	}
	if len(decls) != 1 {
		t.Fatalf("expected 1 cloud workflow declaration, got %d", len(decls))
	}
	if decls[0].Name != "order-flow" {
		t.Fatalf("unexpected workflow name %q", decls[0].Name)
	}

	source := `${bindings.createOrder}`
	got := applyCloudWorkflowBindings(source, decls[0], map[string]string{"createOrder": "https://example.com/create-order"})
	if got != "https://example.com/create-order" {
		t.Fatalf("binding replacement mismatch: got %q", got)
	}
}

func TestCloudWorkflowsFromConfigValidation(t *testing.T) {
	cfg := sdkprovider.Config{
		"extensions": map[string]any{
			"gcp-functions": map[string]any{
				"cloudWorkflows": []any{map[string]any{"name": "missing-definition"}},
			},
		},
	}
	_, err := cloudWorkflowsFromConfig(cfg, ".")
	if err == nil {
		t.Fatal("expected validation error for missing definition")
	}
}

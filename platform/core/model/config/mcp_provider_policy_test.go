package config

import "testing"

func TestParseMCPPolicy_ParsesProviderRules(t *testing.T) {
	cfg := &Config{
		Policies: map[string]any{
			"mcp": map[string]any{
				"providers": map[string]any{
					"aws": map[string]any{
						"requiredRegion":  "us-east-1",
						"requiredAuth":    "iam",
						"denyCrossRegion": true,
						"denyRegions":     []any{"eu-*", "ap-south-1"},
					},
				},
			},
		},
	}

	policy, err := ParseMCPPolicy(cfg)
	if err != nil {
		t.Fatalf("ParseMCPPolicy returned error: %v", err)
	}
	rules, ok := policy.Providers["aws"]
	if !ok {
		t.Fatalf("expected provider policy for aws, got %+v", policy.Providers)
	}
	if rules.RequiredRegion != "us-east-1" {
		t.Fatalf("expected requiredRegion us-east-1, got %q", rules.RequiredRegion)
	}
	if rules.RequiredAuth != "iam" {
		t.Fatalf("expected requiredAuth iam, got %q", rules.RequiredAuth)
	}
	if !rules.DenyCrossRegion {
		t.Fatal("expected denyCrossRegion=true")
	}
	if len(rules.DenyRegions) != 2 || rules.DenyRegions[0] != "eu-*" {
		t.Fatalf("unexpected denyRegions: %+v", rules.DenyRegions)
	}
}

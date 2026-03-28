package config

import "testing"

func TestParseMCPPolicy_ParsesProviderRules(t *testing.T) {
	cfg := &Config{
		Policies: map[string]any{
			"mcp": map[string]any{
				"providers": map[string]any{
					"AWS-LAMBDA": map[string]any{
						"requiredRegion":  "us-east-1",
						"requiredAuth":    "iam",
						"denyCrossRegion": true,
						"denyRegions":     []any{"eu-*", "ap-south-1"},
						"models": map[string]any{
							"default":      "gpt-4.1",
							"ai-generate":  "gpt-4.1",
							"ai-retrieval": "gpt-4.1-mini",
						},
					},
				},
			},
		},
	}

	policy, err := ParseMCPPolicy(cfg)
	if err != nil {
		t.Fatalf("ParseMCPPolicy returned error: %v", err)
	}
	rules, ok := policy.Providers["aws-lambda"]
	if !ok {
		t.Fatalf("expected provider policy for aws-lambda, got %+v", policy.Providers)
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
	if rules.Models["default"] != "gpt-4.1" {
		t.Fatalf("expected default model override, got %+v", rules.Models)
	}
	if rules.Models["ai-retrieval"] != "gpt-4.1-mini" {
		t.Fatalf("expected ai-retrieval model override, got %+v", rules.Models)
	}
}

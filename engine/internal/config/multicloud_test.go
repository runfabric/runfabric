package config

import (
	"testing"
)

func TestApplyProviderOverride_EmptyKeyNoChange(t *testing.T) {
	cfg := &Config{
		Service:  "svc",
		Provider: ProviderConfig{Name: "aws-lambda", Runtime: "nodejs"},
		ProviderOverrides: map[string]ProviderConfig{
			"gcp": {Name: "gcp-functions", Runtime: "nodejs", Region: "us-central1"},
		},
	}
	err := ApplyProviderOverride(cfg, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Provider.Name != "aws-lambda" {
		t.Errorf("expected provider name aws-lambda, got %q", cfg.Provider.Name)
	}
}

func TestApplyProviderOverride_ValidKey(t *testing.T) {
	cfg := &Config{
		Service:  "svc",
		Provider: ProviderConfig{Name: "aws-lambda", Runtime: "nodejs"},
		ProviderOverrides: map[string]ProviderConfig{
			"aws": {Name: "aws-lambda", Runtime: "nodejs", Region: "us-east-1"},
			"gcp": {Name: "gcp-functions", Runtime: "nodejs", Region: "us-central1"},
		},
	}
	err := ApplyProviderOverride(cfg, "gcp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Provider.Name != "gcp-functions" || cfg.Provider.Region != "us-central1" {
		t.Errorf("expected provider gcp-functions us-central1, got %+v", cfg.Provider)
	}
}

func TestApplyProviderOverride_NoProviderOverrides(t *testing.T) {
	cfg := &Config{
		Service:  "svc",
		Provider: ProviderConfig{Name: "aws-lambda", Runtime: "nodejs"},
	}
	err := ApplyProviderOverride(cfg, "gcp")
	if err == nil {
		t.Fatal("expected error when providerOverrides is nil")
	}
}

func TestApplyProviderOverride_UnknownKey(t *testing.T) {
	cfg := &Config{
		Service:  "svc",
		Provider: ProviderConfig{Name: "aws-lambda", Runtime: "nodejs"},
		ProviderOverrides: map[string]ProviderConfig{
			"aws": {Name: "aws-lambda", Runtime: "nodejs"},
		},
	}
	err := ApplyProviderOverride(cfg, "gcp")
	if err == nil {
		t.Fatal("expected error for unknown provider key")
	}
}

func TestListProviderKeys(t *testing.T) {
	if got := ListProviderKeys(&Config{}); got != nil {
		t.Errorf("expected nil for nil ProviderOverrides, got %v", got)
	}
	cfg := &Config{
		ProviderOverrides: map[string]ProviderConfig{
			"aws": {Name: "aws-lambda"},
			"gcp": {Name: "gcp-functions"},
		},
	}
	keys := ListProviderKeys(cfg)
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}
	seen := make(map[string]bool)
	for _, k := range keys {
		seen[k] = true
	}
	if !seen["aws"] || !seen["gcp"] {
		t.Errorf("expected aws and gcp in keys, got %v", keys)
	}
}

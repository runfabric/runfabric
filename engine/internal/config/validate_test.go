package config

import (
	"testing"
)

func TestValidate_RequiresService(t *testing.T) {
	cfg := &Config{Provider: ProviderConfig{Name: "aws", Runtime: "nodejs"}, Functions: map[string]FunctionConfig{"api": {}}}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected error for empty service")
	}
}

func TestValidate_RequiresProviderName(t *testing.T) {
	cfg := &Config{Service: "svc", Provider: ProviderConfig{Runtime: "nodejs"}, Functions: map[string]FunctionConfig{"api": {}}}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected error for empty provider name")
	}
}

func TestValidate_RequiresAtLeastOneFunction(t *testing.T) {
	cfg := &Config{Service: "svc", Provider: ProviderConfig{Name: "aws", Runtime: "nodejs"}, Functions: nil}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected error for no functions")
	}
}

func TestValidate_ValidMinimal(t *testing.T) {
	cfg := &Config{
		Service:   "svc",
		Provider:  ProviderConfig{Name: "aws", Runtime: "nodejs"},
		Functions: map[string]FunctionConfig{"api": {Handler: "src/handler.default"}},
	}
	if err := Validate(cfg); err != nil {
		t.Fatal(err)
	}
}

func TestValidate_BackendS3RequiresBucket(t *testing.T) {
	cfg := &Config{
		Service:   "svc",
		Provider:  ProviderConfig{Name: "aws", Runtime: "nodejs"},
		Functions: map[string]FunctionConfig{"api": {Handler: "h"}},
		Backend:   &BackendConfig{Kind: "s3"},
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected error for s3 without bucket")
	}
}

func TestValidate_DeployStrategy(t *testing.T) {
	base := &Config{
		Service:   "svc",
		Provider:  ProviderConfig{Name: "aws", Runtime: "nodejs"},
		Functions: map[string]FunctionConfig{"api": {Handler: "h"}},
	}
	for _, invalid := range []string{"canary-blue", "rolling", "x"} {
		cfg := *base
		cfg.Deploy = &DeployConfig{Strategy: invalid}
		if err := Validate(&cfg); err == nil {
			t.Errorf("expected error for strategy %q", invalid)
		}
	}
	for _, valid := range []string{"all-at-once", "canary", "blue-green", ""} {
		cfg := *base
		cfg.Deploy = &DeployConfig{Strategy: valid, CanaryPercent: 10}
		if valid == "canary" && cfg.Deploy.CanaryPercent == 10 {
			// 10 is valid
		} else if valid == "canary" {
			cfg.Deploy.CanaryPercent = 10
		}
		if err := Validate(&cfg); err != nil && valid != "" {
			t.Errorf("strategy %q: %v", valid, err)
		}
	}
}

func TestValidate_DeployCanaryPercent(t *testing.T) {
	base := &Config{
		Service:   "svc",
		Provider:  ProviderConfig{Name: "aws", Runtime: "nodejs"},
		Functions: map[string]FunctionConfig{"api": {Handler: "h"}},
		Deploy:    &DeployConfig{Strategy: "canary"},
	}
	cfg := *base
	cfg.Deploy.CanaryPercent = -1
	if err := Validate(&cfg); err == nil {
		t.Error("expected error for canaryPercent -1")
	}
	cfg = *base
	cfg.Deploy.CanaryPercent = 101
	if err := Validate(&cfg); err == nil {
		t.Error("expected error for canaryPercent 101")
	}
	cfg = *base
	cfg.Deploy.CanaryPercent = 50
	if err := Validate(&cfg); err != nil {
		t.Errorf("canaryPercent 50 should be valid: %v", err)
	}
}

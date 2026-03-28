package config

import (
	"testing"
)

func TestValidate_RequiresService(t *testing.T) {
	cfg := &Config{Provider: ProviderConfig{Name: "aws-lambda", Runtime: "nodejs"}, Functions: map[string]FunctionConfig{"api": {}}}
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
	cfg := &Config{Service: "svc", Provider: ProviderConfig{Name: "aws-lambda", Runtime: "nodejs"}, Functions: nil}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected error for no functions")
	}
}

func TestValidate_ValidMinimal(t *testing.T) {
	cfg := &Config{
		Service:   "svc",
		Provider:  ProviderConfig{Name: "aws-lambda", Runtime: "nodejs"},
		Functions: map[string]FunctionConfig{"api": {Handler: "src/handler.default"}},
	}
	if err := Validate(cfg); err != nil {
		t.Fatal(err)
	}
}

func TestValidate_ProviderSourceExternalAllowsVersionPin(t *testing.T) {
	cfg := &Config{
		Service: "svc",
		Provider: ProviderConfig{
			Name:    "vercel",
			Runtime: "nodejs",
			Source:  "external",
			Version: "1.2.3",
		},
		Functions: map[string]FunctionConfig{"api": {Handler: "src/handler.default"}},
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("expected external provider source/version to validate: %v", err)
	}
}

func TestValidate_ProviderVersionRequiresExternalSource(t *testing.T) {
	cfg := &Config{
		Service: "svc",
		Provider: ProviderConfig{
			Name:    "vercel",
			Runtime: "nodejs",
			Version: "1.2.3",
		},
		Functions: map[string]FunctionConfig{"api": {Handler: "src/handler.default"}},
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected error when provider.version is set without provider.source=external")
	}
}

func TestValidate_ProviderSourceRejectsInvalidValues(t *testing.T) {
	cfg := &Config{
		Service: "svc",
		Provider: ProviderConfig{
			Name:    "vercel",
			Runtime: "nodejs",
			Source:  "registry",
		},
		Functions: map[string]FunctionConfig{"api": {Handler: "src/handler.default"}},
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected error for unsupported provider.source")
	}
}

func TestValidate_ProviderSourceExternalAcceptsAWS(t *testing.T) {
	cfg := &Config{
		Service: "svc",
		Provider: ProviderConfig{
			Name:    "aws-lambda",
			Runtime: "nodejs",
			Source:  "external",
		},
		Functions: map[string]FunctionConfig{"api": {Handler: "src/handler.default"}},
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("expected provider.source=external to be accepted for aws-lambda: %v", err)
	}
}

func TestValidate_BackendS3RequiresBucket(t *testing.T) {
	cfg := &Config{
		Service:   "svc",
		Provider:  ProviderConfig{Name: "aws-lambda", Runtime: "nodejs"},
		Functions: map[string]FunctionConfig{"api": {Handler: "h"}},
		Backend:   &BackendConfig{Kind: "s3"},
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected error for s3 without bucket")
	}
}

func TestValidate_BackendPostgresDefaults(t *testing.T) {
	cfg := &Config{
		Service:   "svc",
		Provider:  ProviderConfig{Name: "aws-lambda", Runtime: "nodejs"},
		Functions: map[string]FunctionConfig{"api": {Handler: "h"}},
		Backend:   &BackendConfig{Kind: "postgres"},
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("expected postgres backend to validate with defaults: %v", err)
	}
	if cfg.Backend.PostgresConnectionStringEnv != "RUNFABRIC_STATE_POSTGRES_URL" {
		t.Fatalf("expected default postgres connection env, got %q", cfg.Backend.PostgresConnectionStringEnv)
	}
	if cfg.Backend.PostgresTable != "runfabric_receipts" {
		t.Fatalf("expected default postgres table, got %q", cfg.Backend.PostgresTable)
	}
}

func TestValidate_BackendGCSRequiresStateFields(t *testing.T) {
	cfg := &Config{
		Service:   "svc",
		Provider:  ProviderConfig{Name: "gcp-functions", Runtime: "nodejs"},
		Functions: map[string]FunctionConfig{"api": {Handler: "h"}},
		Backend:   &BackendConfig{Kind: "gcs"},
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected error for gcs without state.gcs config")
	}

	cfg.State = &StateConfig{
		Backend: "gcs",
		GCS:     &StateGCS{Bucket: "state-bucket", Prefix: "rf/state"},
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("expected gcs backend to validate when state.gcs configured: %v", err)
	}
}

func TestValidate_BackendAzblobRequiresStateFields(t *testing.T) {
	cfg := &Config{
		Service:   "svc",
		Provider:  ProviderConfig{Name: "azure-functions", Runtime: "nodejs"},
		Functions: map[string]FunctionConfig{"api": {Handler: "h"}},
		Backend:   &BackendConfig{Kind: "azblob"},
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected error for azblob without state.azblob config")
	}

	cfg.State = &StateConfig{
		Backend: "azblob",
		Azblob:  &StateAzblob{Container: "runfabric-state", Prefix: "rf/state"},
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("expected azblob backend to validate when state.azblob configured: %v", err)
	}
}

func TestValidate_DeployStrategy(t *testing.T) {
	base := &Config{
		Service:   "svc",
		Provider:  ProviderConfig{Name: "aws-lambda", Runtime: "nodejs"},
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
		Provider:  ProviderConfig{Name: "aws-lambda", Runtime: "nodejs"},
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

func minimalValidConfig() *Config {
	return &Config{
		Service:   "svc",
		Provider:  ProviderConfig{Name: "aws-lambda", Runtime: "nodejs"},
		Functions: map[string]FunctionConfig{"api": {Handler: "h"}},
	}
}

func TestValidate_SecretsMapValidation(t *testing.T) {
	cfg := minimalValidConfig()
	cfg.Secrets = map[string]string{"": "x"}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected validation error for empty secrets key")
	}
}

func TestValidate_WorkflowStepKindValidation(t *testing.T) {
	cfg := minimalValidConfig()
	cfg.Workflows = []WorkflowConfig{
		{
			Name: "wf",
			Steps: []WorkflowStep{
				{ID: "s1", Kind: "unknown"},
			},
		},
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected validation error for unsupported workflow step kind")
	}
}

func TestValidate_WorkflowStepInputValidation(t *testing.T) {
	cfg := minimalValidConfig()
	cfg.Workflows = []WorkflowConfig{
		{
			Name: "wf",
			Steps: []WorkflowStep{
				{ID: "retrieve", Kind: "ai-retrieval", Input: map[string]any{}},
			},
		},
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected validation error for ai-retrieval without input.query")
	}
}

func TestValidate_WorkflowStepInputValidation_AcceptsTypedKinds(t *testing.T) {
	cfg := minimalValidConfig()
	cfg.Workflows = []WorkflowConfig{
		{
			Name: "wf",
			Steps: []WorkflowStep{
				{ID: "code", Kind: "code", Input: map[string]any{"function": "deploy"}},
				{ID: "retrieve", Kind: "ai-retrieval", Input: map[string]any{"query": "risks"}},
				{ID: "generate", Kind: "ai-generate", Input: map[string]any{"prompt": "summarize"}},
				{ID: "structured", Kind: "ai-structured", Input: map[string]any{"schema": map[string]any{"type": "object"}}},
				{ID: "eval", Kind: "ai-eval", Input: map[string]any{"score": 0.8, "threshold": 0.5}},
				{ID: "approve", Kind: "human-approval", Input: map[string]any{"approvalDecision": "approved"}},
			},
		},
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("expected workflow typed kinds to validate, got: %v", err)
	}
}

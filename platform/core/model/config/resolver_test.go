package config

import (
	"os"
	"testing"
)

func TestResolve_EnvInterpolation(t *testing.T) {
	os.Setenv("RUNFABRIC_TEST_STAGE", "prod")
	defer os.Unsetenv("RUNFABRIC_TEST_STAGE")
	cfg := &Config{
		Service:  "svc",
		Provider: ProviderConfig{Name: "aws-lambda", Runtime: "nodejs"},
		Functions: map[string]FunctionConfig{
			"api": {Handler: "src/handler", Runtime: "${env:RUNFABRIC_TEST_STAGE}"},
		},
	}
	out, err := Resolve(cfg, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if out.Functions["api"].Runtime != "prod" {
		t.Errorf("expected resolved runtime prod, got %q", out.Functions["api"].Runtime)
	}
}

func TestResolve_SecretInterpolation_FromConfigSecrets(t *testing.T) {
	cfg := &Config{
		Service: "svc",
		Provider: ProviderConfig{
			Name:    "aws-lambda",
			Runtime: "nodejs",
		},
		Secrets: map[string]string{
			"db_password": "s3cr3t",
		},
		Functions: map[string]FunctionConfig{
			"api": {Handler: "src/handler", Runtime: "nodejs20.x", Environment: map[string]string{"DB_PASSWORD": "${secret:db_password}"}},
		},
	}
	out, err := Resolve(cfg, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if got := out.Functions["api"].Environment["DB_PASSWORD"]; got != "s3cr3t" {
		t.Fatalf("DB_PASSWORD=%q want s3cr3t", got)
	}
}

func TestResolve_SecretInterpolation_FromEnv(t *testing.T) {
	os.Setenv("API_TOKEN", "token-123")
	defer os.Unsetenv("API_TOKEN")
	cfg := &Config{
		Service:  "svc",
		Provider: ProviderConfig{Name: "aws-lambda", Runtime: "nodejs"},
		Functions: map[string]FunctionConfig{
			"api": {Handler: "src/handler", Environment: map[string]string{"API_TOKEN": "${secret:API_TOKEN}"}},
		},
	}
	out, err := Resolve(cfg, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if got := out.Functions["api"].Environment["API_TOKEN"]; got != "token-123" {
		t.Fatalf("API_TOKEN=%q want token-123", got)
	}
}

func TestResolve_DoesNotMutateInput(t *testing.T) {
	cfg := &Config{
		Service:  "svc",
		Provider: ProviderConfig{Name: "aws-lambda", Runtime: "nodejs"},
		Functions: map[string]FunctionConfig{
			"api": {Handler: "h", Events: []EventConfig{{HTTP: &HTTPEvent{Path: "/foo", Method: "GET"}}}},
		},
	}
	orig := cfg.Functions["api"].Events[0].HTTP.Path
	_, err := Resolve(cfg, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Functions["api"].Events[0].HTTP.Path != orig {
		t.Error("Resolve mutated original config")
	}
}

func TestEnsureStage(t *testing.T) {
	if err := EnsureStage(""); err == nil {
		t.Fatal("expected error for empty stage")
	}
	if err := EnsureStage("dev"); err != nil {
		t.Fatal(err)
	}
}

func TestResolve_LayerExpansion(t *testing.T) {
	cfg := &Config{
		Service:  "svc",
		Provider: ProviderConfig{Name: "aws-lambda", Runtime: "nodejs"},
		Layers: map[string]LayerConfig{
			"node-deps": {Arn: "arn:aws:lambda:us-east-1:123:layer:node-deps:1"},
		},
		Functions: map[string]FunctionConfig{
			"api": {Handler: "h", Layers: []string{"node-deps", "arn:aws:lambda:us-east-1:456:layer:other:2"}},
		},
	}
	out, err := Resolve(cfg, "dev")
	if err != nil {
		t.Fatal(err)
	}
	got := out.Functions["api"].Layers
	if len(got) != 2 {
		t.Fatalf("expected 2 layers, got %v", got)
	}
	if got[0] != "arn:aws:lambda:us-east-1:123:layer:node-deps:1" {
		t.Errorf("expected first layer to be expanded from node-deps, got %q", got[0])
	}
	if got[1] != "arn:aws:lambda:us-east-1:456:layer:other:2" {
		t.Errorf("expected second layer to be literal ARN, got %q", got[1])
	}
}

func TestResolve_LayerVersionFromEnv(t *testing.T) {
	os.Setenv("LAYER_VER", "42")
	defer os.Unsetenv("LAYER_VER")
	cfg := &Config{
		Service:  "svc",
		Provider: ProviderConfig{Name: "aws-lambda", Runtime: "nodejs"},
		Layers: map[string]LayerConfig{
			"node-deps": {Arn: "arn:aws:lambda:us-east-1:123:layer:node-deps:1", Version: "${env:LAYER_VER}"},
		},
		Functions: map[string]FunctionConfig{
			"api": {Handler: "h", Layers: []string{"node-deps"}},
		},
	}
	out, err := Resolve(cfg, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if out.Layers["node-deps"].Version != "42" {
		t.Errorf("expected layer version resolved from env to 42, got %q", out.Layers["node-deps"].Version)
	}
}

func TestResolve_BuildOrderAndAlertsAndAppOrg(t *testing.T) {
	os.Setenv("BUILD_STEP", "compile")
	os.Setenv("ALERT_URL", "https://hooks.example.com/alert")
	os.Setenv("MY_APP", "my-app")
	defer func() {
		os.Unsetenv("BUILD_STEP")
		os.Unsetenv("ALERT_URL")
		os.Unsetenv("MY_APP")
	}()
	cfg := &Config{
		Service:  "svc",
		Provider: ProviderConfig{Name: "aws-lambda", Runtime: "nodejs"},
		App:      "${env:MY_APP}",
		Build:    &BuildConfig{Order: []string{"deps", "${env:BUILD_STEP}"}},
		Alerts:   &AlertsConfig{Webhook: "${env:ALERT_URL}", OnError: true},
		Functions: map[string]FunctionConfig{
			"api": {Handler: "h"},
		},
	}
	out, err := Resolve(cfg, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if out.App != "my-app" {
		t.Errorf("expected app resolved to my-app, got %q", out.App)
	}
	if out.Build == nil || len(out.Build.Order) != 2 || out.Build.Order[1] != "compile" {
		t.Errorf("expected build.order[1] resolved to compile, got %v", out.Build)
	}
	if out.Alerts == nil || out.Alerts.Webhook != "https://hooks.example.com/alert" || !out.Alerts.OnError {
		t.Errorf("expected alerts.webhook and onError resolved, got %v", out.Alerts)
	}
}

func TestResolve_ScalingDefaults(t *testing.T) {
	cfg := &Config{
		Service:  "svc",
		Provider: ProviderConfig{Name: "aws-lambda", Runtime: "nodejs"},
		Deploy: &DeployConfig{
			Scaling: &ScalingConfig{ReservedConcurrency: 10, ProvisionedConcurrency: 1},
		},
		Functions: map[string]FunctionConfig{
			"api":    {Handler: "h"},                         // gets defaults
			"worker": {Handler: "w", ReservedConcurrency: 5}, // overrides reserved only
		},
	}
	out, err := Resolve(cfg, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if out.Functions["api"].ReservedConcurrency != 10 || out.Functions["api"].ProvisionedConcurrency != 1 {
		t.Errorf("api: expected reserved=10 provisioned=1, got reserved=%d provisioned=%d",
			out.Functions["api"].ReservedConcurrency, out.Functions["api"].ProvisionedConcurrency)
	}
	if out.Functions["worker"].ReservedConcurrency != 5 || out.Functions["worker"].ProvisionedConcurrency != 1 {
		t.Errorf("worker: expected reserved=5 provisioned=1, got reserved=%d provisioned=%d",
			out.Functions["worker"].ReservedConcurrency, out.Functions["worker"].ProvisionedConcurrency)
	}
}

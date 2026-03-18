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

func minimalValidConfig() *Config {
	return &Config{
		Service:   "svc",
		Provider:  ProviderConfig{Name: "aws", Runtime: "nodejs"},
		Functions: map[string]FunctionConfig{"api": {Handler: "h"}},
	}
}

func TestValidate_AiWorkflow_EnableRequiresNodes(t *testing.T) {
	cfg := minimalValidConfig()
	cfg.AiWorkflow = &AiWorkflowConfig{Enable: true, Nodes: nil}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected error when aiWorkflow.enable is true and nodes empty")
	}
}

func TestValidate_AiWorkflow_DuplicateNodeID(t *testing.T) {
	cfg := minimalValidConfig()
	cfg.AiWorkflow = &AiWorkflowConfig{
		Enable: true,
		Nodes: []AiWorkflowNode{
			{ID: "n1", Type: "trigger"},
			{ID: "n1", Type: "ai"},
		},
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected error for duplicate node id")
	}
}

func TestValidate_AiWorkflow_InvalidNodeType(t *testing.T) {
	cfg := minimalValidConfig()
	cfg.AiWorkflow = &AiWorkflowConfig{
		Enable: true,
		Nodes:  []AiWorkflowNode{{ID: "n1", Type: "unknown"}},
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected error for unsupported node type")
	}
}

func TestValidate_AiWorkflow_EntrypointMustBeNode(t *testing.T) {
	cfg := minimalValidConfig()
	cfg.AiWorkflow = &AiWorkflowConfig{
		Enable:     true,
		Entrypoint: "missing",
		Nodes:      []AiWorkflowNode{{ID: "n1", Type: "trigger"}},
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected error when entrypoint is not a node id")
	}
}

func TestValidate_AiWorkflow_EdgeEndpointsMustBeNodes(t *testing.T) {
	cfg := minimalValidConfig()
	cfg.AiWorkflow = &AiWorkflowConfig{
		Enable: true,
		Nodes:  []AiWorkflowNode{{ID: "n1", Type: "trigger"}},
		Edges:  []AiWorkflowEdge{{From: "n1", To: "n2"}},
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected error when edge to is not a node id")
	}
}

func TestValidate_AiWorkflow_Valid(t *testing.T) {
	cfg := minimalValidConfig()
	cfg.AiWorkflow = &AiWorkflowConfig{
		Enable:     true,
		Entrypoint: "start",
		Nodes: []AiWorkflowNode{
			{ID: "start", Type: "trigger"},
			{ID: "step1", Type: "ai"},
		},
		Edges: []AiWorkflowEdge{{From: "start", To: "step1"}},
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("valid aiWorkflow should pass: %v", err)
	}
}

func TestValidate_AiWorkflow_DisabledNoValidation(t *testing.T) {
	cfg := minimalValidConfig()
	cfg.AiWorkflow = &AiWorkflowConfig{Enable: false, Nodes: nil}
	if err := Validate(cfg); err != nil {
		t.Fatalf("aiWorkflow disabled with no nodes should not error: %v", err)
	}
}

func TestCompileAiWorkflow(t *testing.T) {
	g, err := CompileAiWorkflow(nil)
	if err != nil || g != nil {
		t.Fatalf("CompileAiWorkflow(nil) want nil,nil got %v, %v", g, err)
	}
	cfg := &AiWorkflowConfig{
		Enable:     true,
		Entrypoint: "start",
		Nodes: []AiWorkflowNode{
			{ID: "start", Type: "trigger"},
			{ID: "step1", Type: "ai"},
		},
		Edges: []AiWorkflowEdge{{From: "start", To: "step1"}},
	}
	g, err = CompileAiWorkflow(cfg)
	if err != nil {
		t.Fatalf("CompileAiWorkflow: %v", err)
	}
	if g == nil || g.Entrypoint != "start" || len(g.Order) != 2 || g.Hash == "" {
		t.Errorf("CompileAiWorkflow: got Entrypoint=%q Order=%v Hash=%q", g.Entrypoint, g.Order, g.Hash)
	}
}

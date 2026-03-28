package controlplane

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/runfabric/runfabric/platform/core/model/config"
	state "github.com/runfabric/runfabric/platform/core/state/core"
)

func TestProviderPromptRenderer_AWS(t *testing.T) {
	r := ProviderPromptRenderer("aws")
	out := r.Render(PromptRenderInput{
		BasePrompt: "draft",
		MCPPrompt:  "ctx",
		Step:       state.WorkflowStepRun{StepID: "s1", Kind: StepKindAIGenerate},
		Run:        &state.WorkflowRun{RunID: "r1", WorkflowHash: "wf1"},
	})
	if !strings.Contains(out, "provider:bedrock") || !strings.Contains(out, "[system]") {
		t.Fatalf("expected aws bedrock prompt envelope, got %q", out)
	}
}

func TestProviderPromptRenderer_GCP(t *testing.T) {
	r := ProviderPromptRenderer("gcp")
	out := r.Render(PromptRenderInput{Step: state.WorkflowStepRun{StepID: "s1", Kind: StepKindAIGenerate}})
	if !strings.Contains(out, "provider:vertex") {
		t.Fatalf("expected gcp vertex prompt envelope, got %q", out)
	}
}

func TestProviderPromptRenderer_Azure(t *testing.T) {
	r := ProviderPromptRenderer("azure")
	out := r.Render(PromptRenderInput{Step: state.WorkflowStepRun{StepID: "s1", Kind: StepKindAIGenerate}})
	if !strings.Contains(out, "provider:azure-openai") {
		t.Fatalf("expected azure prompt envelope, got %q", out)
	}
}

func TestProviderToolResultMapper_AWS(t *testing.T) {
	m := ProviderToolResultMapper("aws")
	out := m.MapToolResult("crm", map[string]any{
		"toolResult": map[string]any{"content": []any{map[string]any{"text": "ok"}}},
	})
	if out["provider"] != "aws" || out["result"] != "ok" {
		t.Fatalf("unexpected mapped aws result: %+v", out)
	}
}

func TestProviderToolResultMapper_GCP(t *testing.T) {
	m := ProviderToolResultMapper("gcp")
	out := m.MapToolResult("crm", map[string]any{
		"functionResponse": map[string]any{"name": "lookup", "response": map[string]any{"id": "1"}},
	})
	if out["provider"] != "gcp" {
		t.Fatalf("expected gcp provider, got %+v", out)
	}
	res, _ := out["result"].(map[string]any)
	if res["id"] != "1" {
		t.Fatalf("unexpected gcp mapped response: %+v", out)
	}
}

func TestProviderToolResultMapper_Azure(t *testing.T) {
	m := ProviderToolResultMapper("azure")
	out := m.MapToolResult("crm", map[string]any{"content": "done"})
	if out["provider"] != "azure" || out["result"] != "done" {
		t.Fatalf("unexpected azure mapped response: %+v", out)
	}
}

func TestProviderModelOutputShaper_AWS(t *testing.T) {
	s := ProviderModelOutputShaper("aws")
	out := s.ShapeOutput(StepKindAIGenerate, "s1", map[string]any{"text": "hello", "model": "custom-model"})
	if out["provider"] != "aws" || out["stopReason"] == nil {
		t.Fatalf("expected aws-shaped output, got %+v", out)
	}
	if out["model"] != "custom-model" {
		t.Fatalf("expected selected model override to be preserved, got %+v", out)
	}
}

func TestProviderModelOutputShaper_GCP(t *testing.T) {
	s := ProviderModelOutputShaper("gcp")
	out := s.ShapeOutput(StepKindAIGenerate, "s1", map[string]any{"text": "hello", "model": "custom-model"})
	if out["provider"] != "gcp" || out["usageMetadata"] == nil {
		t.Fatalf("expected gcp-shaped output, got %+v", out)
	}
	if out["model"] != "custom-model" {
		t.Fatalf("expected selected model override to be preserved, got %+v", out)
	}
}

func TestProviderModelOutputShaper_Azure(t *testing.T) {
	s := ProviderModelOutputShaper("azure")
	out := s.ShapeOutput(StepKindAIGenerate, "s1", map[string]any{"text": "hello", "model": "custom-model"})
	if out["provider"] != "azure" || out["usage"] == nil {
		t.Fatalf("expected azure-shaped output, got %+v", out)
	}
	if out["model"] != "custom-model" {
		t.Fatalf("expected selected model override to be preserved, got %+v", out)
	}
}

func TestProviderTelemetryHook_NoError(t *testing.T) {
	run := &state.WorkflowRun{RunID: "r1", WorkflowHash: "wf"}
	step := state.WorkflowStepRun{StepID: "s1", Kind: StepKindAIGenerate}
	res := &StepExecutionResult{Output: map[string]any{}, Metadata: map[string]any{}}

	hooks := []StepTelemetryHook{
		ProviderTelemetryHook("aws", "us-east-1", ""),
		ProviderTelemetryHook("gcp", "us-central1", "proj"),
		ProviderTelemetryHook("azure", "eastus", ""),
	}
	for _, h := range hooks {
		h.RecordStep(run, step, res, 50*time.Millisecond, nil)
		h.RecordStep(run, step, res, 50*time.Millisecond, errors.New("boom"))
	}
}

func TestProviderRetryStrategy_Behavior(t *testing.T) {
	aws := ProviderRetryStrategy("aws")
	if !aws.ShouldRetry(1, errors.New("ThrottlingException")) {
		t.Fatal("expected aws throttling retry")
	}
	if aws.Backoff(1) <= 0 {
		t.Fatal("expected positive aws backoff")
	}

	gcp := ProviderRetryStrategy("gcp")
	if !gcp.ShouldRetry(1, errors.New("RESOURCE_EXHAUSTED")) {
		t.Fatal("expected gcp resource exhausted retry")
	}

	azure := ProviderRetryStrategy("azure")
	if !azure.ShouldRetry(1, errors.New("429")) {
		t.Fatal("expected azure rate-limit retry")
	}

	def := ProviderRetryStrategy("unknown")
	if !def.ShouldRetry(1, errors.New("any")) {
		t.Fatal("expected default retry for non-nil error")
	}
}

func TestProviderModelSelector_RoutesByProvider(t *testing.T) {
	if got := ProviderModelSelector("aws").SelectModel(StepKindAIGenerate, "us-east-1"); !strings.Contains(got, "sonnet") {
		t.Fatalf("expected aws sonnet in supported region, got %q", got)
	}
	if got := ProviderModelSelector("gcp").SelectModel(StepKindAIRetrieval, ""); !strings.Contains(got, "flash") {
		t.Fatalf("expected gcp flash for retrieval, got %q", got)
	}
	if got := ProviderModelSelector("azure").SelectModel(StepKindAIGenerate, ""); got != "gpt-4o" {
		t.Fatalf("expected azure gpt-4o for generate, got %q", got)
	}
}

func TestWithModelSelectorOverrides_UsesOverridesAndFallsBack(t *testing.T) {
	selector := WithModelSelectorOverrides(DefaultModelSelector{}, map[string]string{
		"ai-generate": "gpt-4.1",
		"default":     "gpt-4.1-mini",
	})
	if got := selector.SelectModel(StepKindAIGenerate, ""); got != "gpt-4.1" {
		t.Fatalf("expected ai-generate override, got %q", got)
	}
	if got := selector.SelectModel(StepKindAIEval, ""); got != "gpt-4.1-mini" {
		t.Fatalf("expected default override for ai-eval, got %q", got)
	}
}

func TestEnvModelSelectorOverrides_ProviderOverridesGlobal(t *testing.T) {
	t.Setenv("RUNFABRIC_MODEL_AI_GENERATE", "global-generate")
	t.Setenv("RUNFABRIC_MODEL_AWS_AI_GENERATE", "aws-generate")
	t.Setenv("RUNFABRIC_MODEL_DEFAULT", "global-default")
	t.Setenv("RUNFABRIC_MODEL_AWS_DEFAULT", "aws-default")

	overrides := EnvModelSelectorOverrides("aws")
	if got := overrides[StepKindAIGenerate]; got != "aws-generate" {
		t.Fatalf("expected provider-scoped ai-generate override, got %q", got)
	}
	if got := overrides["default"]; got != "aws-default" {
		t.Fatalf("expected provider-scoped default override, got %q", got)
	}
}

func TestProviderCacheKeyGenerator_StableScoped(t *testing.T) {
	aws := ProviderCacheKeyGenerator("aws", "us-east-1", "", "")
	k1 := aws.CacheKey("crm", "tool", "lookup", map[string]any{"id": "1", "name": "a"})
	k2 := aws.CacheKey("crm", "tool", "lookup", map[string]any{"name": "a", "id": "1"})
	if k1 != k2 {
		t.Fatalf("expected stable key regardless of arg ordering: %q vs %q", k1, k2)
	}

	gcp := ProviderCacheKeyGenerator("gcp", "", "proj-a", "")
	k3 := gcp.CacheKey("crm", "tool", "lookup", map[string]any{"id": "1"})
	if k3 == k1 {
		t.Fatalf("expected provider/project scoping to produce different keys: %q == %q", k3, k1)
	}
}

func TestProviderCostTracker_RecordsTotals(t *testing.T) {
	trackers := []CostTracker{
		ProviderCostTracker("aws"),
		ProviderCostTracker("gcp"),
		ProviderCostTracker("azure"),
	}
	for _, tr := range trackers {
		tr.RecordCost("", "model", 1000, 500)
		if tr.TotalCostUSD() <= 0 {
			t.Fatalf("expected positive total cost, got %v", tr.TotalCostUSD())
		}
		s := tr.Summary()
		if s["recordCount"] == nil {
			t.Fatalf("expected recordCount in summary, got %+v", s)
		}
	}
}

func TestNewTypedStepHandlerFromConfig_InjectsProviderComponents(t *testing.T) {
	cfg := &config.Config{
		Provider: config.ProviderConfig{Name: "aws", Region: "us-east-1"},
		Integrations: map[string]any{
			"mcp": map[string]any{
				"servers": map[string]any{
					"crm": map[string]any{"url": "https://crm.internal/mcp"},
				},
			},
		},
	}
	h, err := NewTypedStepHandlerFromConfig(cfg, fakeMCPClient{})
	if err != nil {
		t.Fatalf("NewTypedStepHandlerFromConfig returned error: %v", err)
	}
	runner, ok := h.AIRunner.(*DefaultAIStepRunner)
	if !ok {
		t.Fatalf("expected default ai runner type, got %T", h.AIRunner)
	}
	if runner.MCPRuntime == nil || runner.MCPRuntime.Provider != "aws" || runner.MCPRuntime.ActiveRegion != "us-east-1" {
		t.Fatalf("expected aws provider context in runtime, got %+v", runner.MCPRuntime)
	}
	if _, ok := runner.PromptRenderer.(AWSBedrockPromptRenderer); !ok {
		t.Fatalf("expected AWSBedrockPromptRenderer, got %T", runner.PromptRenderer)
	}
	if _, ok := runner.ToolMapper.(AWSToolResultMapper); !ok {
		t.Fatalf("expected AWSToolResultMapper, got %T", runner.ToolMapper)
	}
	if _, ok := runner.OutputShaper.(AWSBedrockOutputShaper); !ok {
		t.Fatalf("expected AWSBedrockOutputShaper, got %T", runner.OutputShaper)
	}
	if _, ok := runner.RetryStrategy.(AWSRetryStrategy); !ok {
		t.Fatalf("expected AWSRetryStrategy, got %T", runner.RetryStrategy)
	}
	if _, ok := runner.CostTracker.(*AWSCostTracker); !ok {
		t.Fatalf("expected AWSCostTracker, got %T", runner.CostTracker)
	}
	if _, ok := h.TelemetryHook.(AWSCloudWatchHook); !ok {
		t.Fatalf("expected AWSCloudWatchHook, got %T", h.TelemetryHook)
	}
}

func TestNewTypedStepHandlerFromConfig_AppliesProviderModelOverrides(t *testing.T) {
	cfg := &config.Config{
		Provider: config.ProviderConfig{Name: "aws", Region: "us-east-1"},
		Policies: map[string]any{
			"mcp": map[string]any{
				"providers": map[string]any{
					"aws": map[string]any{
						"models": map[string]any{
							"default":      "gpt-4.1-mini",
							"ai-generate":  "gpt-4.1",
							"ai-retrieval": "gpt-4.1-mini",
						},
					},
				},
			},
		},
	}
	h, err := NewTypedStepHandlerFromConfig(cfg, fakeMCPClient{})
	if err != nil {
		t.Fatalf("NewTypedStepHandlerFromConfig returned error: %v", err)
	}
	runner, ok := h.AIRunner.(*DefaultAIStepRunner)
	if !ok {
		t.Fatalf("expected default ai runner type, got %T", h.AIRunner)
	}
	if got := runner.ModelSelector.SelectModel(StepKindAIGenerate, "us-east-1"); got != "gpt-4.1" {
		t.Fatalf("expected ai-generate override model, got %q", got)
	}
	if got := runner.ModelSelector.SelectModel(StepKindAIEval, "us-east-1"); got != "gpt-4.1-mini" {
		t.Fatalf("expected default override model for ai-eval, got %q", got)
	}
}

func TestNewTypedStepHandlerFromConfig_ModelOverridePrecedence_ConfigOverEnv(t *testing.T) {
	t.Setenv("RUNFABRIC_MODEL_AWS_AI_GENERATE", "env-model")
	cfg := &config.Config{
		Provider: config.ProviderConfig{Name: "aws", Region: "us-east-1"},
		Policies: map[string]any{
			"mcp": map[string]any{
				"providers": map[string]any{
					"aws": map[string]any{
						"models": map[string]any{
							"ai-generate": "config-model",
						},
					},
				},
			},
		},
	}
	h, err := NewTypedStepHandlerFromConfig(cfg, fakeMCPClient{})
	if err != nil {
		t.Fatalf("NewTypedStepHandlerFromConfig returned error: %v", err)
	}
	runner, ok := h.AIRunner.(*DefaultAIStepRunner)
	if !ok {
		t.Fatalf("expected default ai runner type, got %T", h.AIRunner)
	}
	if got := runner.ModelSelector.SelectModel(StepKindAIGenerate, "us-east-1"); got != "config-model" {
		t.Fatalf("expected config model to override env model, got %q", got)
	}
}

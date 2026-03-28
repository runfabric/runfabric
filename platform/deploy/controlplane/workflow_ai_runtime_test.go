package controlplane

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/runfabric/runfabric/platform/core/model/config"
	state "github.com/runfabric/runfabric/platform/core/state/core"
)

type failingMCPClient struct {
	err error
}

func (f failingMCPClient) CallTool(_ context.Context, _, _ string, _ map[string]any) (map[string]any, error) {
	return nil, f.err
}

func (f failingMCPClient) ReadResource(_ context.Context, _, _ string) (map[string]any, error) {
	return nil, f.err
}

func (f failingMCPClient) GetPrompt(_ context.Context, _, _ string, _ map[string]any) (map[string]any, error) {
	return nil, f.err
}

type fakeAIStepRunner struct {
	called bool
}

func (f *fakeAIStepRunner) ExecuteStep(_ context.Context, _ *state.WorkflowRun, _ state.WorkflowStepRun, output, metadata map[string]any) (*StepExecutionResult, error) {
	f.called = true
	output["boundary"] = "ai_runner"
	metadata["runner"] = "fake"
	return &StepExecutionResult{Output: output, Metadata: metadata}, nil
}

func TestTypedStepHandler_UsesAIRunnerBoundary(t *testing.T) {
	h := NewTypedStepHandler(config.MCPIntegrationsConfig{}, config.MCPPolicyConfig{}, nil)
	fake := &fakeAIStepRunner{}
	h.AIRunner = fake

	run := &state.WorkflowRun{RunID: "r1", WorkflowHash: "wf1"}
	step := state.WorkflowStepRun{StepID: "s1", Kind: StepKindAIGenerate, Input: map[string]any{"prompt": "hello"}}
	res, err := h.ExecuteStep(context.Background(), run, step)
	if err != nil {
		t.Fatalf("ExecuteStep returned error: %v", err)
	}
	if !fake.called {
		t.Fatal("expected typed handler to call AIRunner boundary")
	}
	if res.Output["boundary"] != "ai_runner" {
		t.Fatalf("expected ai_runner boundary marker, got %+v", res.Output)
	}
}

func TestDeterministicPromptRenderer_ComposesPromptWithContexts(t *testing.T) {
	renderer := DeterministicPromptRenderer{}
	out := renderer.Render(PromptRenderInput{
		BasePrompt: "draft release note",
		MCPPrompt:  "customer context",
		Step:       state.WorkflowStepRun{StepID: "s-generate", Kind: StepKindAIGenerate},
		Run:        &state.WorkflowRun{RunID: "run-100", WorkflowHash: "wf-hash-1"},
	})
	if !strings.Contains(out, "MCP Prompt:\ncustomer context") {
		t.Fatalf("expected mcp prompt in rendered output, got: %s", out)
	}
	if !strings.Contains(out, "Base Prompt:\ndraft release note") {
		t.Fatalf("expected base prompt in rendered output, got: %s", out)
	}
	if !strings.Contains(out, "stepId=s-generate") || !strings.Contains(out, "runId=run-100") {
		t.Fatalf("expected step/run context in rendered output, got: %s", out)
	}
}

func TestDefaultAIStepRunner_AIGenerate_ShapesOutputAndMergesToolResult(t *testing.T) {
	runner := NewDefaultAIStepRunner(
		NewMCPRuntime(
			fakeMCPClient{},
			config.MCPIntegrationsConfig{Servers: map[string]config.MCPServerConfig{"crm": {URL: "https://crm.internal/mcp"}}},
			config.MCPPolicyConfig{
				DefaultDeny: true,
				Allow: config.MCPPolicyRuleSet{
					Tools:   []string{"crm.lookup*"},
					Prompts: []string{"crm.greeting*"},
				},
			},
		),
		DeterministicPromptRenderer{},
	)

	run := &state.WorkflowRun{RunID: "run-1", WorkflowHash: "wf-abc"}
	step := state.WorkflowStepRun{
		StepID: "s1",
		Kind:   StepKindAIGenerate,
		Input: map[string]any{
			"prompt": "draft a reply",
			"mcp": map[string]any{
				"server":     "crm",
				"prompt":     "greeting.template",
				"promptArgs": map[string]any{"name": "alice"},
				"tool":       "lookup_customer",
				"toolArgs":   map[string]any{"id": "123"},
			},
		},
	}
	res, err := runner.ExecuteStep(context.Background(), run, step, map[string]any{"kind": StepKindAIGenerate}, map[string]any{})
	if err != nil {
		t.Fatalf("ExecuteStep returned error: %v", err)
	}
	if _, ok := res.Output["mcpTool"]; !ok {
		t.Fatalf("expected merged mcpTool output, got %+v", res.Output)
	}
	if toolResults, ok := res.Output["toolResults"].([]any); !ok || len(toolResults) != 1 {
		t.Fatalf("expected one tool result, got %+v", res.Output["toolResults"])
	}
	modelOutput, ok := res.Output["modelOutput"].(map[string]any)
	if !ok || modelOutput["type"] != "text" {
		t.Fatalf("expected shaped text model output, got %+v", res.Output["modelOutput"])
	}
	if !strings.Contains(asInputString(res.Output, "text"), "MCP Prompt:") {
		t.Fatalf("expected rendered prompt pipeline in output text, got %q", asInputString(res.Output, "text"))
	}
}

func TestDefaultAIStepRunner_MCPDenied(t *testing.T) {
	runner := NewDefaultAIStepRunner(
		NewMCPRuntime(
			fakeMCPClient{},
			config.MCPIntegrationsConfig{Servers: map[string]config.MCPServerConfig{"kb": {URL: "https://kb.internal/mcp"}}},
			config.MCPPolicyConfig{DefaultDeny: true},
		),
		nil,
	)

	run := &state.WorkflowRun{RunID: "run-2", WorkflowHash: "wf-xyz"}
	step := state.WorkflowStepRun{
		StepID: "s2",
		Kind:   StepKindAIRetrieval,
		Input: map[string]any{
			"query": "refund policy",
			"mcp": map[string]any{
				"server":   "kb",
				"resource": "kb://policies/refund",
			},
		},
	}
	_, err := runner.ExecuteStep(context.Background(), run, step, map[string]any{"kind": StepKindAIRetrieval}, map[string]any{})
	if err == nil {
		t.Fatal("expected mcp policy deny error")
	}
	if !strings.Contains(err.Error(), "defaultDeny") {
		t.Fatalf("expected defaultDeny reason in error, got: %v", err)
	}
}

func TestDefaultAIStepRunner_PropagatesMCPFailure(t *testing.T) {
	runner := NewDefaultAIStepRunner(
		NewMCPRuntime(
			failingMCPClient{err: errors.New("mcp transport failure")},
			config.MCPIntegrationsConfig{Servers: map[string]config.MCPServerConfig{"crm": {URL: "https://crm.internal/mcp"}}},
			config.MCPPolicyConfig{Allow: config.MCPPolicyRuleSet{Tools: []string{"crm.lookup*"}}},
		),
		nil,
	)

	run := &state.WorkflowRun{RunID: "run-3", WorkflowHash: "wf-err"}
	step := state.WorkflowStepRun{
		StepID: "s3",
		Kind:   StepKindAIRetrieval,
		Input: map[string]any{
			"query": "customer details",
			"mcp": map[string]any{
				"server": "crm",
				"tool":   "lookup_customer",
			},
		},
	}
	_, err := runner.ExecuteStep(context.Background(), run, step, map[string]any{"kind": StepKindAIRetrieval}, map[string]any{})
	if err == nil {
		t.Fatal("expected MCP client failure to propagate")
	}
	if !strings.Contains(err.Error(), "mcp transport failure") {
		t.Fatalf("expected propagated MCP failure, got %v", err)
	}
}

func TestDefaultAIStepRunner_PerStepModelOverride_TakesPrecedence(t *testing.T) {
	runner := NewDefaultAIStepRunner(NewMCPRuntime(fakeMCPClient{}, config.MCPIntegrationsConfig{}, config.MCPPolicyConfig{}), nil)
	runner.ModelSelector = WithModelSelectorOverrides(DefaultModelSelector{}, map[string]string{
		"default": "selector-model",
	})

	run := &state.WorkflowRun{RunID: "run-step-model", WorkflowHash: "wf-step-model"}
	step := state.WorkflowStepRun{
		StepID: "s-model",
		Kind:   StepKindAIEval,
		Input: map[string]any{
			"score": 0.9,
			"model": "step-model",
		},
	}

	res, err := runner.ExecuteStep(context.Background(), run, step, map[string]any{"kind": StepKindAIEval}, map[string]any{})
	if err != nil {
		t.Fatalf("ExecuteStep returned error: %v", err)
	}
	if got, _ := res.Metadata["selectedModel"].(string); got != "step-model" {
		t.Fatalf("expected per-step model override, got %q", got)
	}
}

func TestDefaultAIStepRunner_AIEval_ShaperUsesSelectedModel(t *testing.T) {
	runner := NewDefaultAIStepRunner(NewMCPRuntime(fakeMCPClient{}, config.MCPIntegrationsConfig{}, config.MCPPolicyConfig{}), nil)
	runner.OutputShaper = AWSBedrockOutputShaper{}
	runner.ModelSelector = WithModelSelectorOverrides(DefaultModelSelector{}, map[string]string{
		"default": "selector-model",
	})

	run := &state.WorkflowRun{RunID: "run-eval-model", WorkflowHash: "wf-eval-model"}
	step := state.WorkflowStepRun{
		StepID: "s-eval",
		Kind:   StepKindAIEval,
		Input: map[string]any{
			"score": 0.9,
			"model": "step-model",
		},
	}

	res, err := runner.ExecuteStep(context.Background(), run, step, map[string]any{"kind": StepKindAIEval}, map[string]any{})
	if err != nil {
		t.Fatalf("ExecuteStep returned error: %v", err)
	}
	modelOutput, _ := res.Output["modelOutput"].(map[string]any)
	if got, _ := modelOutput["model"].(string); got != "step-model" {
		t.Fatalf("expected shaper model to match selected model, got %q", got)
	}
}

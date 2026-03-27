package controlplane

import (
	"context"
	"strings"
	"testing"

	"github.com/runfabric/runfabric/platform/core/model/config"
	state "github.com/runfabric/runfabric/platform/core/state/core"
)

type fakeMCPClient struct{}

func (fakeMCPClient) CallTool(_ context.Context, server, name string, args map[string]any) (map[string]any, error) {
	return map[string]any{"server": server, "name": name, "args": args}, nil
}

func (fakeMCPClient) ReadResource(_ context.Context, server, uri string) (map[string]any, error) {
	return map[string]any{"server": server, "uri": uri, "content": "resource-content"}, nil
}

func (fakeMCPClient) GetPrompt(_ context.Context, server, ref string, args map[string]any) (map[string]any, error) {
	return map[string]any{"server": server, "ref": ref, "text": "prompt-from-mcp", "args": args}, nil
}

func TestTypedStepHandler_AIGenerate_LogsMCPCorrelation(t *testing.T) {
	handler := NewTypedStepHandler(
		config.MCPIntegrationsConfig{
			Servers: map[string]config.MCPServerConfig{
				"crm": {URL: "https://crm.internal/mcp"},
			},
		},
		config.MCPPolicyConfig{
			DefaultDeny: true,
			Allow: config.MCPPolicyRuleSet{
				Tools:   []string{"crm.lookup*"},
				Prompts: []string{"crm.greeting*"},
			},
		},
		fakeMCPClient{},
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
	res, err := handler.ExecuteStep(context.Background(), run, step)
	if err != nil {
		t.Fatalf("ExecuteStep returned error: %v", err)
	}
	if res == nil || res.Output == nil {
		t.Fatal("expected non-nil execution result output")
	}
	text, _ := res.Output["text"].(string)
	if !strings.Contains(text, "generated(s1)") {
		t.Fatalf("expected generated text envelope, got %q", text)
	}
	calls, ok := res.Metadata["mcpCalls"].([]any)
	if !ok || len(calls) != 2 {
		t.Fatalf("expected two mcp call correlation records, got %+v", res.Metadata["mcpCalls"])
	}
	first, _ := calls[0].(map[string]any)
	if first["runId"] != "run-1" || first["stepId"] != "s1" || first["workflowHash"] != "wf-abc" {
		t.Fatalf("unexpected correlation metadata: %+v", first)
	}
}

func TestTypedStepHandler_MCPDefaultDeny(t *testing.T) {
	handler := NewTypedStepHandler(
		config.MCPIntegrationsConfig{
			Servers: map[string]config.MCPServerConfig{
				"kb": {URL: "https://kb.internal/mcp"},
			},
		},
		config.MCPPolicyConfig{
			DefaultDeny: true,
		},
		fakeMCPClient{},
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
	_, err := handler.ExecuteStep(context.Background(), run, step)
	if err == nil {
		t.Fatal("expected mcp default deny error")
	}
	if !strings.Contains(err.Error(), "defaultDeny") {
		t.Fatalf("expected defaultDeny error message, got %v", err)
	}
}

func TestTypedStepHandler_AIStructured_RequiresSchema(t *testing.T) {
	handler := NewTypedStepHandler(config.MCPIntegrationsConfig{}, config.MCPPolicyConfig{}, nil)
	run := &state.WorkflowRun{RunID: "run-3", WorkflowHash: "wf"}
	step := state.WorkflowStepRun{
		StepID: "s3",
		Kind:   StepKindAIStructured,
		Input:  map[string]any{"data": map[string]any{"a": 1}},
	}
	_, err := handler.ExecuteStep(context.Background(), run, step)
	if err == nil {
		t.Fatal("expected schema validation error for ai-structured step")
	}
}

func TestWorkflowRuntime_HumanApprovalPauseDecisionResume(t *testing.T) {
	root := t.TempDir()
	handler := NewTypedStepHandler(config.MCPIntegrationsConfig{}, config.MCPPolicyConfig{}, nil)
	rt := NewWorkflowRuntime(root, handler)

	run, err := rt.StartRun(context.Background(), WorkflowRunSpec{
		RunID:        "approval-run-1",
		Service:      "svc",
		Stage:        "dev",
		WorkflowHash: "wf-1",
		Entrypoint:   "approval",
		Steps: []WorkflowStepSpec{
			{
				ID:    "approval",
				Kind:  StepKindHumanApproval,
				Input: map[string]any{"approvalRequest": "review refund"},
			},
		},
	})
	if err != nil {
		t.Fatalf("StartRun returned error: %v", err)
	}
	if run.Status != state.RunStatusPaused {
		t.Fatalf("expected paused run after human approval step, got %q", run.Status)
	}
	if run.Steps[0].Status != state.StepStatusPaused {
		t.Fatalf("expected paused step status, got %q", run.Steps[0].Status)
	}

	if err := rt.ResolveApproval("dev", "approval-run-1", "approval", "approved", "alice"); err != nil {
		t.Fatalf("ResolveApproval returned error: %v", err)
	}
	resumed, err := rt.ResumeRun(context.Background(), "dev", "approval-run-1")
	if err != nil {
		t.Fatalf("ResumeRun returned error: %v", err)
	}
	if resumed.Status != state.RunStatusOK {
		t.Fatalf("expected ok status after approval resume, got %q", resumed.Status)
	}
	if resumed.Steps[0].Status != state.StepStatusOK {
		t.Fatalf("expected approved step to reach ok status, got %q", resumed.Steps[0].Status)
	}
	if resumed.Steps[0].Output["status"] != "approved" {
		t.Fatalf("expected approved output envelope, got %+v", resumed.Steps[0].Output)
	}
}

func TestNewTypedStepHandlerFromConfig_WiresMCP(t *testing.T) {
	cfg := &config.Config{
		Integrations: map[string]any{
			"mcp": map[string]any{
				"servers": map[string]any{
					"crm": map[string]any{"url": "https://crm.internal/mcp"},
				},
			},
		},
		Policies: map[string]any{
			"mcp": map[string]any{
				"defaultDeny": true,
				"allow": map[string]any{
					"tools": []any{"crm.lookup*"},
				},
			},
		},
	}
	h, err := NewTypedStepHandlerFromConfig(cfg, fakeMCPClient{})
	if err != nil {
		t.Fatalf("NewTypedStepHandlerFromConfig returned error: %v", err)
	}
	if _, ok := h.Integrations.Servers["crm"]; !ok {
		t.Fatalf("expected crm integration server wiring, got %+v", h.Integrations.Servers)
	}
	if !h.Policy.DefaultDeny {
		t.Fatalf("expected default deny policy wiring")
	}
}

// --- Code step path ---

func TestTypedStepHandler_CodeStep_ExecutesAndEchoesInput(t *testing.T) {
	handler := NewTypedStepHandler(config.MCPIntegrationsConfig{}, config.MCPPolicyConfig{}, nil)
	run := &state.WorkflowRun{RunID: "run-code", WorkflowHash: "wf-code"}
	step := state.WorkflowStepRun{
		StepID: "do-work",
		Kind:   StepKindCode,
		Input:  map[string]any{"function": "processOrder", "orderId": "42"},
	}
	res, err := handler.ExecuteStep(context.Background(), run, step)
	if err != nil {
		t.Fatalf("code step returned error: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil result")
	}
	if res.Output["result"] != "code_executed" {
		t.Fatalf("expected result=code_executed, got %v", res.Output["result"])
	}
	input, _ := res.Output["input"].(map[string]any)
	if input["function"] != "processOrder" {
		t.Fatalf("expected function echoed in output, got %+v", input)
	}
}

func TestTypedStepHandler_CodeStep_InjectableRunner(t *testing.T) {
	handler := NewTypedStepHandler(config.MCPIntegrationsConfig{}, config.MCPPolicyConfig{}, nil)
	var executedInput map[string]any
	handler.CodeRunner = &customCodeRunner{captureInput: &executedInput}

	run := &state.WorkflowRun{RunID: "run-custom-code", WorkflowHash: "wf"}
	step := state.WorkflowStepRun{
		StepID: "cs1",
		Kind:   StepKindCode,
		Input:  map[string]any{"cmd": "build"},
	}
	_, err := handler.ExecuteStep(context.Background(), run, step)
	if err != nil {
		t.Fatalf("custom code runner returned error: %v", err)
	}
	if executedInput["cmd"] != "build" {
		t.Fatalf("expected injected runner to receive step input, got %+v", executedInput)
	}
}

type customCodeRunner struct {
	captureInput *map[string]any
}

func (r *customCodeRunner) ExecuteStep(_ *state.WorkflowRun, step state.WorkflowStepRun, output, metadata map[string]any) (*StepExecutionResult, error) {
	*r.captureInput = step.Input
	output["result"] = "custom_executed"
	return &StepExecutionResult{Output: output, Metadata: metadata}, nil
}

// --- Human-approval step ---

func TestTypedStepHandler_HumanApproval_PausesWithNoDecision(t *testing.T) {
	handler := NewTypedStepHandler(config.MCPIntegrationsConfig{}, config.MCPPolicyConfig{}, nil)
	run := &state.WorkflowRun{RunID: "appr-run", WorkflowHash: "wf"}
	step := state.WorkflowStepRun{
		StepID: "review",
		Kind:   StepKindHumanApproval,
		Input:  map[string]any{"approvalRequest": "please review this PR"},
	}
	res, err := handler.ExecuteStep(context.Background(), run, step)
	if err != nil {
		t.Fatalf("human approval step returned error: %v", err)
	}
	if !res.Pause {
		t.Fatal("expected Pause=true when no approval decision supplied")
	}
	if res.Output["status"] != "awaiting_approval" {
		t.Fatalf("expected awaiting_approval status, got %v", res.Output["status"])
	}
}

func TestTypedStepHandler_HumanApproval_RejectPath(t *testing.T) {
	handler := NewTypedStepHandler(config.MCPIntegrationsConfig{}, config.MCPPolicyConfig{}, nil)
	run := &state.WorkflowRun{RunID: "appr-run-2", WorkflowHash: "wf"}
	step := state.WorkflowStepRun{
		StepID: "review",
		Kind:   StepKindHumanApproval,
		Input:  map[string]any{"approvalDecision": "reject", "approvalReviewer": "bob"},
	}
	res, err := handler.ExecuteStep(context.Background(), run, step)
	if err != nil {
		t.Fatalf("human approval rejection returned error: %v", err)
	}
	if res.Output["status"] != "rejected" {
		t.Fatalf("expected rejected status, got %v", res.Output["status"])
	}
	if res.Output["reviewer"] != "bob" {
		t.Fatalf("expected reviewer=bob, got %v", res.Output["reviewer"])
	}
}

func TestTypedStepHandler_HumanApproval_InvalidDecisionErrors(t *testing.T) {
	handler := NewTypedStepHandler(config.MCPIntegrationsConfig{}, config.MCPPolicyConfig{}, nil)
	run := &state.WorkflowRun{RunID: "appr-run-3", WorkflowHash: "wf"}
	step := state.WorkflowStepRun{
		StepID: "review",
		Kind:   StepKindHumanApproval,
		Input:  map[string]any{"approvalDecision": "maybe"},
	}
	_, err := handler.ExecuteStep(context.Background(), run, step)
	if err == nil {
		t.Fatal("expected error for invalid approval decision")
	}
	if !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("expected invalid decision error, got %v", err)
	}
}

func TestTypedStepHandler_HumanApproval_InjectableRunner(t *testing.T) {
	handler := NewTypedStepHandler(config.MCPIntegrationsConfig{}, config.MCPPolicyConfig{}, nil)
	var called bool
	handler.ApprovalRunner = &customApprovalRunner{called: &called}

	run := &state.WorkflowRun{RunID: "appr-custom", WorkflowHash: "wf"}
	step := state.WorkflowStepRun{
		StepID: "review",
		Kind:   StepKindHumanApproval,
		Input:  map[string]any{"approvalDecision": "approved"},
	}
	_, err := handler.ExecuteStep(context.Background(), run, step)
	if err != nil {
		t.Fatalf("custom approval runner returned error: %v", err)
	}
	if !called {
		t.Fatal("expected custom ApprovalRunner to be invoked")
	}
}

type customApprovalRunner struct {
	called *bool
}

func (r *customApprovalRunner) ExecuteStep(_ *state.WorkflowRun, _ state.WorkflowStepRun, output, metadata map[string]any) (*StepExecutionResult, error) {
	*r.called = true
	output["status"] = "approved"
	return &StepExecutionResult{Output: output, Metadata: metadata}, nil
}

// --- AI structured step ---

func TestTypedStepHandler_AIStructured_ProducesObjectOutput(t *testing.T) {
	handler := NewTypedStepHandler(config.MCPIntegrationsConfig{}, config.MCPPolicyConfig{}, nil)
	run := &state.WorkflowRun{RunID: "run-struct", WorkflowHash: "wf"}
	step := state.WorkflowStepRun{
		StepID: "extract",
		Kind:   StepKindAIStructured,
		Input: map[string]any{
			"schema": map[string]any{"type": "object", "properties": map[string]any{"name": "string"}},
			"data":   map[string]any{"name": "alice"},
		},
	}
	res, err := handler.ExecuteStep(context.Background(), run, step)
	if err != nil {
		t.Fatalf("ai-structured step returned error: %v", err)
	}
	if _, ok := res.Output["object"]; !ok {
		t.Fatalf("expected object field in output, got %+v", res.Output)
	}
	if _, ok := res.Output["schema"]; !ok {
		t.Fatalf("expected schema field in output, got %+v", res.Output)
	}
	mo, _ := res.Output["modelOutput"].(map[string]any)
	if mo["type"] != "object" {
		t.Fatalf("expected modelOutput.type=object, got %v", mo["type"])
	}
}

// --- AI eval step ---

func TestTypedStepHandler_AIEval_PassAboveThreshold(t *testing.T) {
	handler := NewTypedStepHandler(config.MCPIntegrationsConfig{}, config.MCPPolicyConfig{}, nil)
	run := &state.WorkflowRun{RunID: "run-eval-pass", WorkflowHash: "wf"}
	step := state.WorkflowStepRun{
		StepID: "quality-gate",
		Kind:   StepKindAIEval,
		Input:  map[string]any{"score": 0.9, "threshold": 0.7},
	}
	res, err := handler.ExecuteStep(context.Background(), run, step)
	if err != nil {
		t.Fatalf("ai-eval step returned error: %v", err)
	}
	pass, _ := res.Output["pass"].(bool)
	if !pass {
		t.Fatalf("expected pass=true for score 0.9 >= threshold 0.7, got output %+v", res.Output)
	}
}

func TestTypedStepHandler_AIEval_FailBelowThreshold(t *testing.T) {
	handler := NewTypedStepHandler(config.MCPIntegrationsConfig{}, config.MCPPolicyConfig{}, nil)
	run := &state.WorkflowRun{RunID: "run-eval-fail", WorkflowHash: "wf"}
	step := state.WorkflowStepRun{
		StepID: "quality-gate",
		Kind:   StepKindAIEval,
		Input:  map[string]any{"score": 0.3, "threshold": 0.7},
	}
	res, err := handler.ExecuteStep(context.Background(), run, step)
	if err != nil {
		t.Fatalf("ai-eval step returned error: %v", err)
	}
	pass, _ := res.Output["pass"].(bool)
	if pass {
		t.Fatalf("expected pass=false for score 0.3 < threshold 0.7, got output %+v", res.Output)
	}
}

func TestTypedStepHandler_AIEval_DefaultThreshold(t *testing.T) {
	handler := NewTypedStepHandler(config.MCPIntegrationsConfig{}, config.MCPPolicyConfig{}, nil)
	run := &state.WorkflowRun{RunID: "run-eval-default", WorkflowHash: "wf"}
	// No threshold supplied; default is 0.5.
	step := state.WorkflowStepRun{
		StepID: "gate",
		Kind:   StepKindAIEval,
		Input:  map[string]any{"score": 0.6},
	}
	res, err := handler.ExecuteStep(context.Background(), run, step)
	if err != nil {
		t.Fatalf("ai-eval default threshold returned error: %v", err)
	}
	pass, _ := res.Output["pass"].(bool)
	if !pass {
		t.Fatalf("expected pass=true with score 0.6 >= default threshold 0.5")
	}
}

// --- AI retrieval + MCP resource ---

func TestTypedStepHandler_AIRetrieval_WithMCPResource(t *testing.T) {
	handler := NewTypedStepHandler(
		config.MCPIntegrationsConfig{
			Servers: map[string]config.MCPServerConfig{
				"kb": {URL: "https://kb.internal/mcp"},
			},
		},
		config.MCPPolicyConfig{Allow: config.MCPPolicyRuleSet{Resources: []string{"kb.*"}}},
		fakeMCPClient{},
	)
	run := &state.WorkflowRun{RunID: "run-retrieval", WorkflowHash: "wf"}
	step := state.WorkflowStepRun{
		StepID: "fetch-docs",
		Kind:   StepKindAIRetrieval,
		Input: map[string]any{
			"query": "what is the refund policy",
			"mcp": map[string]any{
				"server":   "kb",
				"resource": "kb://policies/refund",
			},
		},
	}
	res, err := handler.ExecuteStep(context.Background(), run, step)
	if err != nil {
		t.Fatalf("ai-retrieval with mcp resource returned error: %v", err)
	}
	if res.Output["query"] != "what is the refund policy" {
		t.Fatalf("expected query echoed in output, got %+v", res.Output)
	}
	if _, ok := res.Output["documents"]; !ok {
		t.Fatalf("expected documents in output, got %+v", res.Output)
	}
}

// --- End-to-end state transitions (WorkflowRuntime) ---

func TestWorkflowRuntime_CodeStepEndToEnd(t *testing.T) {
	root := t.TempDir()
	handler := NewTypedStepHandler(config.MCPIntegrationsConfig{}, config.MCPPolicyConfig{}, nil)
	rt := NewWorkflowRuntime(root, handler)

	run, err := rt.StartRun(context.Background(), WorkflowRunSpec{
		RunID:        "e2e-code-1",
		Service:      "svc",
		Stage:        "dev",
		WorkflowHash: "wf-e2e",
		Entrypoint:   "step1",
		Steps: []WorkflowStepSpec{
			{ID: "step1", Kind: StepKindCode, Input: map[string]any{"fn": "a"}},
			{ID: "step2", Kind: StepKindCode, Input: map[string]any{"fn": "b"}},
		},
	})
	if err != nil {
		t.Fatalf("StartRun returned error: %v", err)
	}
	if run.Status != state.RunStatusOK {
		t.Fatalf("expected ok status after two code steps, got %q", run.Status)
	}
	if len(run.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(run.Steps))
	}
	for _, s := range run.Steps {
		if s.Status != state.StepStatusOK {
			t.Fatalf("expected all steps ok, step %q has status %q", s.StepID, s.Status)
		}
	}
}

func TestWorkflowRuntime_AIStepEndToEnd(t *testing.T) {
	root := t.TempDir()
	handler := NewTypedStepHandler(config.MCPIntegrationsConfig{}, config.MCPPolicyConfig{}, nil)
	rt := NewWorkflowRuntime(root, handler)

	run, err := rt.StartRun(context.Background(), WorkflowRunSpec{
		RunID:        "e2e-ai-1",
		Service:      "svc",
		Stage:        "dev",
		WorkflowHash: "wf-e2e",
		Entrypoint:   "generate",
		Steps: []WorkflowStepSpec{
			{ID: "generate", Kind: StepKindAIGenerate, Input: map[string]any{"prompt": "summarise this document"}},
		},
	})
	if err != nil {
		t.Fatalf("AI generate step e2e returned error: %v", err)
	}
	if run.Status != state.RunStatusOK {
		t.Fatalf("expected ok status, got %q", run.Status)
	}
	step := run.Steps[0]
	if step.Status != state.StepStatusOK {
		t.Fatalf("expected ai-generate step ok, got %q", step.Status)
	}
	if _, ok := step.Output["text"]; !ok {
		t.Fatalf("expected text in ai-generate output, got %+v", step.Output)
	}
}

func TestWorkflowRuntime_HumanApprovalFullLifecycle(t *testing.T) {
	root := t.TempDir()
	handler := NewTypedStepHandler(config.MCPIntegrationsConfig{}, config.MCPPolicyConfig{}, nil)
	rt := NewWorkflowRuntime(root, handler)

	run, err := rt.StartRun(context.Background(), WorkflowRunSpec{
		RunID:        "e2e-approval-1",
		Service:      "svc",
		Stage:        "dev",
		WorkflowHash: "wf-e2e",
		Entrypoint:   "pre",
		Steps: []WorkflowStepSpec{
			{ID: "pre", Kind: StepKindCode, Input: map[string]any{}},
			{ID: "review", Kind: StepKindHumanApproval, Input: map[string]any{}},
			{ID: "post", Kind: StepKindCode, Input: map[string]any{}},
		},
	})
	if err != nil {
		t.Fatalf("StartRun returned error: %v", err)
	}
	if run.Status != state.RunStatusPaused {
		t.Fatalf("expected paused run awaiting approval, got %q", run.Status)
	}

	// Resolve and resume.
	if err := rt.ResolveApproval("dev", "e2e-approval-1", "review", "approved", "reviewer-1"); err != nil {
		t.Fatalf("ResolveApproval returned error: %v", err)
	}
	completed, err := rt.ResumeRun(context.Background(), "dev", "e2e-approval-1")
	if err != nil {
		t.Fatalf("ResumeRun after approval returned error: %v", err)
	}
	if completed.Status != state.RunStatusOK {
		t.Fatalf("expected ok status after approval completion, got %q", completed.Status)
	}
	for _, s := range completed.Steps {
		if s.Status != state.StepStatusOK {
			t.Fatalf("expected all steps ok after completion, step %q has %q", s.StepID, s.Status)
		}
	}
}

func TestWorkflowRuntime_MixedStepsStateTransitions(t *testing.T) {
	root := t.TempDir()
	handler := NewTypedStepHandler(config.MCPIntegrationsConfig{}, config.MCPPolicyConfig{}, nil)
	rt := NewWorkflowRuntime(root, handler)

	run, err := rt.StartRun(context.Background(), WorkflowRunSpec{
		RunID:        "e2e-mixed-1",
		Service:      "svc",
		Stage:        "dev",
		WorkflowHash: "wf-e2e",
		Entrypoint:   "ingest",
		Steps: []WorkflowStepSpec{
			{ID: "ingest", Kind: StepKindCode, Input: map[string]any{"fn": "ingest"}},
			{ID: "eval", Kind: StepKindAIEval, Input: map[string]any{"score": 0.8, "threshold": 0.5}},
			{ID: "summarise", Kind: StepKindAIGenerate, Input: map[string]any{"prompt": "summarise"}},
		},
	})
	if err != nil {
		t.Fatalf("mixed step run returned error: %v", err)
	}
	if run.Status != state.RunStatusOK {
		t.Fatalf("expected ok status, got %q", run.Status)
	}
	for _, s := range run.Steps {
		if s.Status != state.StepStatusOK {
			t.Fatalf("step %q has status %q", s.StepID, s.Status)
		}
	}
	// Confirm eval passed.
	evalOut := run.Steps[1].Output
	if evalOut["pass"] != true {
		t.Fatalf("expected eval step to pass, got %+v", evalOut)
	}
}

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

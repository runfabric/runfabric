package controlplane

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/runfabric/runfabric/platform/core/model/config"
	state "github.com/runfabric/runfabric/platform/core/state/core"
)

const (
	StepKindCode          = "code"
	StepKindAIRetrieval   = "ai-retrieval"
	StepKindAIGenerate    = "ai-generate"
	StepKindAIStructured  = "ai-structured"
	StepKindAIEval        = "ai-eval"
	StepKindHumanApproval = "human-approval"
)

// NoopMCPClient is a deterministic MCP client for local runtime/testing.
type NoopMCPClient struct{}

func (NoopMCPClient) CallTool(_ context.Context, server, name string, args map[string]any) (map[string]any, error) {
	return map[string]any{
		"type":   "tool",
		"server": server,
		"name":   name,
		"args":   args,
	}, nil
}

func (NoopMCPClient) ReadResource(_ context.Context, server, uri string) (map[string]any, error) {
	return map[string]any{
		"type":   "resource",
		"server": server,
		"uri":    uri,
		"value":  fmt.Sprintf("resource:%s", uri),
	}, nil
}

func (NoopMCPClient) GetPrompt(_ context.Context, server, ref string, args map[string]any) (map[string]any, error) {
	return map[string]any{
		"type":   "prompt",
		"server": server,
		"ref":    ref,
		"text":   fmt.Sprintf("prompt:%s", ref),
		"args":   args,
	}, nil
}

// CodeStepRunner executes code step kinds.
type CodeStepRunner interface {
	ExecuteStep(run *state.WorkflowRun, step state.WorkflowStepRun, output, metadata map[string]any) (*StepExecutionResult, error)
}

// DefaultCodeStepRunner is the built-in code step executor (stub: invocation is provider/runtime-specific).
type DefaultCodeStepRunner struct{}

func (DefaultCodeStepRunner) ExecuteStep(_ *state.WorkflowRun, step state.WorkflowStepRun, output, metadata map[string]any) (*StepExecutionResult, error) {
	output["result"] = "code_executed"
	output["input"] = step.Input
	_ = metadata
	return &StepExecutionResult{Output: output, Metadata: metadata}, nil
}

// ApprovalStepRunner executes human-approval step kinds.
type ApprovalStepRunner interface {
	ExecuteStep(run *state.WorkflowRun, step state.WorkflowStepRun, output, metadata map[string]any) (*StepExecutionResult, error)
}

// DefaultApprovalStepRunner implements pause-on-first-call / decision-on-resume human approval.
type DefaultApprovalStepRunner struct{}

func (DefaultApprovalStepRunner) ExecuteStep(run *state.WorkflowRun, step state.WorkflowStepRun, output, metadata map[string]any) (*StepExecutionResult, error) {
	decision := strings.ToLower(strings.TrimSpace(asInputString(step.Input, "approvalDecision")))
	if decision == "" {
		output["status"] = "awaiting_approval"
		return &StepExecutionResult{
			Output:      output,
			Metadata:    metadata,
			Pause:       true,
			PauseReason: "awaiting human approval",
		}, nil
	}
	switch decision {
	case "approve", "approved", "reject", "rejected":
	default:
		return nil, fmt.Errorf("step %s human approval decision %q is invalid (use approve/approved/reject/rejected)", step.StepID, decision)
	}
	output["status"] = "approved"
	if strings.HasPrefix(decision, "reject") {
		output["status"] = "rejected"
	}
	output["decision"] = decision
	output["reviewer"] = asInputString(step.Input, "approvalReviewer")
	output["runId"] = run.RunID
	return &StepExecutionResult{Output: output, Metadata: metadata}, nil
}

type TypedStepHandler struct {
	MCPClient      MCPClient
	Integrations   config.MCPIntegrationsConfig
	Policy         config.MCPPolicyConfig
	AIRunner       AIStepRunner
	CodeRunner     CodeStepRunner
	ApprovalRunner ApprovalStepRunner
	Now            func() time.Time
	// TelemetryHook records step execution metrics to provider monitoring. Optional.
	TelemetryHook StepTelemetryHook
}

func NewTypedStepHandler(integrations config.MCPIntegrationsConfig, policy config.MCPPolicyConfig, mcp MCPClient) *TypedStepHandler {
	if mcp == nil {
		mcp = NoopMCPClient{}
	}
	mcpRuntime := NewMCPRuntime(mcp, integrations, policy)
	aiRunner := NewDefaultAIStepRunner(mcpRuntime, DeterministicPromptRenderer{})
	return &TypedStepHandler{
		MCPClient:      mcp,
		Integrations:   integrations,
		Policy:         policy,
		AIRunner:       aiRunner,
		CodeRunner:     DefaultCodeStepRunner{},
		ApprovalRunner: DefaultApprovalStepRunner{},
		TelemetryHook:  NoopTelemetryHook{},
		Now:            time.Now,
	}
}

func NewTypedStepHandlerFromConfig(cfg *config.Config, mcp MCPClient) (*TypedStepHandler, error) {
	integrations, err := config.ParseMCPIntegrations(cfg)
	if err != nil {
		return nil, err
	}
	policy, err := config.ParseMCPPolicy(cfg)
	if err != nil {
		return nil, err
	}
	h := NewTypedStepHandler(integrations, policy, mcp)
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider.Name))
	region := strings.TrimSpace(cfg.Provider.Region)
	if provider != "" {
		h.TelemetryHook = ProviderTelemetryHook(provider, region, "")
		if runner, ok := h.AIRunner.(*DefaultAIStepRunner); ok {
			if runner.MCPRuntime != nil {
				runner.MCPRuntime.Provider = provider
				runner.MCPRuntime.ActiveRegion = region
			}
			runner.PromptRenderer = ProviderPromptRenderer(provider)
			runner.ToolMapper = ProviderToolResultMapper(provider)
			runner.OutputShaper = ProviderModelOutputShaper(provider)
			runner.ModelSelector = ProviderModelSelector(provider)
			runner.RetryStrategy = ProviderRetryStrategy(provider)
			runner.CostTracker = ProviderCostTracker(provider)
		}
	}
	return h, nil
}

func (h *TypedStepHandler) ExecuteStep(ctx context.Context, run *state.WorkflowRun, step state.WorkflowStepRun) (*StepExecutionResult, error) {
	kind := strings.ToLower(strings.TrimSpace(step.Kind))
	if kind == "" {
		kind = StepKindCode
	}

	output := map[string]any{
		"kind":   kind,
		"stepId": step.StepID,
	}
	metadata := map[string]any{
		"kind": kind,
		"correlation": map[string]any{
			"runId":        run.RunID,
			"stepId":       step.StepID,
			"workflowHash": run.WorkflowHash,
		},
	}

	start := h.Now()
	var result *StepExecutionResult
	var err error

	switch kind {
	case StepKindCode:
		if h.CodeRunner == nil {
			return nil, fmt.Errorf("code step runner is not configured")
		}
		result, err = h.CodeRunner.ExecuteStep(run, step, output, metadata)
	case StepKindAIRetrieval, StepKindAIGenerate, StepKindAIStructured, StepKindAIEval:
		if h.AIRunner == nil {
			return nil, fmt.Errorf("ai step runner is not configured")
		}
		result, err = h.AIRunner.ExecuteStep(ctx, run, step, output, metadata)
	case StepKindHumanApproval:
		if h.ApprovalRunner == nil {
			return nil, fmt.Errorf("approval step runner is not configured")
		}
		result, err = h.ApprovalRunner.ExecuteStep(run, step, output, metadata)
	default:
		return nil, fmt.Errorf("unsupported workflow step kind %q (allowed: code, ai-retrieval, ai-generate, ai-structured, ai-eval, human-approval)", step.Kind)
	}

	if h.TelemetryHook != nil {
		h.TelemetryHook.RecordStep(run, step, result, h.Now().Sub(start), err)
	}
	return result, err
}

func appendMCPCorrelation(metadata map[string]any, typ, server, target string, run *state.WorkflowRun, step state.WorkflowStepRun) {
	record := map[string]any{
		"type":         typ,
		"server":       server,
		"target":       target,
		"runId":        run.RunID,
		"stepId":       step.StepID,
		"workflowHash": run.WorkflowHash,
	}
	raw := metadata["mcpCalls"]
	if raw == nil {
		metadata["mcpCalls"] = []any{record}
		return
	}
	arr, ok := raw.([]any)
	if !ok {
		metadata["mcpCalls"] = []any{record}
		return
	}
	metadata["mcpCalls"] = append(arr, record)
}

func ruleSetByAction(set config.MCPPolicyRuleSet, action string) []string {
	switch action {
	case "tool":
		return set.Tools
	case "resource":
		return set.Resources
	case "prompt":
		return set.Prompts
	default:
		return nil
	}
}

func wildcardMatch(pattern, value string) bool {
	p := strings.TrimSpace(pattern)
	v := strings.TrimSpace(value)
	if p == "" {
		return false
	}
	if p == "*" || p == v {
		return true
	}
	if strings.HasSuffix(p, "*") {
		prefix := strings.TrimSuffix(p, "*")
		return strings.HasPrefix(v, prefix)
	}
	return false
}

func asInputString(obj map[string]any, key string) string {
	if obj == nil {
		return ""
	}
	s, _ := obj[key].(string)
	return strings.TrimSpace(s)
}

func asFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case int32:
		return float64(n), true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
		if err == nil {
			return f, true
		}
	}
	return 0, false
}

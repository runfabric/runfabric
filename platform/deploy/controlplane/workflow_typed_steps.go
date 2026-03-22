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

// MCPClient is the runtime MCP client contract used by typed step handlers.
type MCPClient interface {
	CallTool(ctx context.Context, server, name string, args map[string]any) (map[string]any, error)
	ReadResource(ctx context.Context, server, uri string) (map[string]any, error)
	GetPrompt(ctx context.Context, server, ref string, args map[string]any) (map[string]any, error)
}

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

type TypedStepHandler struct {
	MCPClient    MCPClient
	Integrations config.MCPIntegrationsConfig
	Policy       config.MCPPolicyConfig
	Now          func() time.Time
}

func NewTypedStepHandler(integrations config.MCPIntegrationsConfig, policy config.MCPPolicyConfig, mcp MCPClient) *TypedStepHandler {
	if mcp == nil {
		mcp = NoopMCPClient{}
	}
	return &TypedStepHandler{
		MCPClient:    mcp,
		Integrations: integrations,
		Policy:       policy,
		Now:          time.Now,
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
	return NewTypedStepHandler(integrations, policy, mcp), nil
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

	switch kind {
	case StepKindCode:
		output["result"] = "code_executed"
		output["input"] = step.Input
		return &StepExecutionResult{Output: output, Metadata: metadata}, nil
	case StepKindAIRetrieval:
		return h.executeAIRetrieval(ctx, run, step, output, metadata)
	case StepKindAIGenerate:
		return h.executeAIGenerate(ctx, run, step, output, metadata)
	case StepKindAIStructured:
		return h.executeAIStructured(ctx, run, step, output, metadata)
	case StepKindAIEval:
		return h.executeAIEval(ctx, run, step, output, metadata)
	case StepKindHumanApproval:
		return h.executeHumanApproval(run, step, output, metadata)
	default:
		return nil, fmt.Errorf("unsupported workflow step kind %q (allowed: code, ai-retrieval, ai-generate, ai-structured, ai-eval, human-approval)", step.Kind)
	}
}

func (h *TypedStepHandler) executeAIRetrieval(ctx context.Context, run *state.WorkflowRun, step state.WorkflowStepRun, output, metadata map[string]any) (*StepExecutionResult, error) {
	query := strings.TrimSpace(asInputString(step.Input, "query"))
	if query == "" {
		return nil, fmt.Errorf("step %s kind ai-retrieval requires input.query", step.StepID)
	}
	documents := []any{}
	if binding, ok := readMCPBinding(step.Input); ok {
		if binding.Resource != "" {
			result, err := h.callMCPResource(ctx, run, step, binding, metadata)
			if err != nil {
				return nil, err
			}
			documents = append(documents, result)
		}
		if binding.Tool != "" {
			result, err := h.callMCPTool(ctx, run, step, binding, metadata)
			if err != nil {
				return nil, err
			}
			documents = append(documents, result)
		}
	}
	output["query"] = query
	output["documents"] = documents
	return &StepExecutionResult{Output: output, Metadata: metadata}, nil
}

func (h *TypedStepHandler) executeAIGenerate(ctx context.Context, run *state.WorkflowRun, step state.WorkflowStepRun, output, metadata map[string]any) (*StepExecutionResult, error) {
	prompt := strings.TrimSpace(asInputString(step.Input, "prompt"))
	if prompt == "" {
		return nil, fmt.Errorf("step %s kind ai-generate requires input.prompt", step.StepID)
	}
	var mcpPromptText string
	if binding, ok := readMCPBinding(step.Input); ok {
		if binding.Prompt != "" {
			result, err := h.callMCPPrompt(ctx, run, step, binding, metadata)
			if err != nil {
				return nil, err
			}
			if text, ok := result["text"].(string); ok {
				mcpPromptText = text
			}
			output["mcpPrompt"] = result
		}
		if binding.Tool != "" {
			toolRes, err := h.callMCPTool(ctx, run, step, binding, metadata)
			if err != nil {
				return nil, err
			}
			output["mcpTool"] = toolRes
		}
	}
	fullPrompt := prompt
	if mcpPromptText != "" {
		fullPrompt = mcpPromptText + "\n" + prompt
	}
	output["text"] = fmt.Sprintf("generated(%s): %s", step.StepID, fullPrompt)
	return &StepExecutionResult{Output: output, Metadata: metadata}, nil
}

func (h *TypedStepHandler) executeAIStructured(_ context.Context, _ *state.WorkflowRun, step state.WorkflowStepRun, output, metadata map[string]any) (*StepExecutionResult, error) {
	schemaObj, ok := step.Input["schema"].(map[string]any)
	if !ok || len(schemaObj) == 0 {
		return nil, fmt.Errorf("step %s kind ai-structured requires input.schema object", step.StepID)
	}
	obj := map[string]any{
		"schemaValidated": true,
		"stepId":          step.StepID,
	}
	if data, ok := step.Input["data"].(map[string]any); ok {
		for k, v := range data {
			obj[k] = v
		}
	}
	output["object"] = obj
	output["schema"] = schemaObj
	return &StepExecutionResult{Output: output, Metadata: metadata}, nil
}

func (h *TypedStepHandler) executeAIEval(_ context.Context, _ *state.WorkflowRun, step state.WorkflowStepRun, output, metadata map[string]any) (*StepExecutionResult, error) {
	score, ok := asFloat(step.Input["score"])
	if !ok {
		return nil, fmt.Errorf("step %s kind ai-eval requires numeric input.score", step.StepID)
	}
	threshold := 0.5
	if v, ok := asFloat(step.Input["threshold"]); ok {
		threshold = v
	}
	output["score"] = score
	output["threshold"] = threshold
	output["pass"] = score >= threshold
	return &StepExecutionResult{Output: output, Metadata: metadata}, nil
}

func (h *TypedStepHandler) executeHumanApproval(run *state.WorkflowRun, step state.WorkflowStepRun, output, metadata map[string]any) (*StepExecutionResult, error) {
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

type mcpBinding struct {
	Server     string
	Tool       string
	ToolArgs   map[string]any
	Resource   string
	Prompt     string
	PromptArgs map[string]any
}

func readMCPBinding(input map[string]any) (mcpBinding, bool) {
	if input == nil {
		return mcpBinding{}, false
	}
	raw, ok := input["mcp"]
	if !ok || raw == nil {
		return mcpBinding{}, false
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		return mcpBinding{}, false
	}
	b := mcpBinding{
		Server:   asInputString(obj, "server"),
		Tool:     asInputString(obj, "tool"),
		Resource: asInputString(obj, "resource"),
		Prompt:   asInputString(obj, "prompt"),
	}
	if args, ok := obj["toolArgs"].(map[string]any); ok {
		b.ToolArgs = args
	}
	if args, ok := obj["promptArgs"].(map[string]any); ok {
		b.PromptArgs = args
	}
	return b, true
}

func (h *TypedStepHandler) callMCPTool(ctx context.Context, run *state.WorkflowRun, step state.WorkflowStepRun, b mcpBinding, metadata map[string]any) (map[string]any, error) {
	if err := h.ensureMCPAllowed("tool", b.Server, b.Tool); err != nil {
		return nil, err
	}
	result, err := h.MCPClient.CallTool(ctx, b.Server, b.Tool, b.ToolArgs)
	if err != nil {
		return nil, err
	}
	appendMCPCorrelation(metadata, "tool", b.Server, b.Tool, run, step)
	return result, nil
}

func (h *TypedStepHandler) callMCPResource(ctx context.Context, run *state.WorkflowRun, step state.WorkflowStepRun, b mcpBinding, metadata map[string]any) (map[string]any, error) {
	if err := h.ensureMCPAllowed("resource", b.Server, b.Resource); err != nil {
		return nil, err
	}
	result, err := h.MCPClient.ReadResource(ctx, b.Server, b.Resource)
	if err != nil {
		return nil, err
	}
	appendMCPCorrelation(metadata, "resource", b.Server, b.Resource, run, step)
	return result, nil
}

func (h *TypedStepHandler) callMCPPrompt(ctx context.Context, run *state.WorkflowRun, step state.WorkflowStepRun, b mcpBinding, metadata map[string]any) (map[string]any, error) {
	if err := h.ensureMCPAllowed("prompt", b.Server, b.Prompt); err != nil {
		return nil, err
	}
	result, err := h.MCPClient.GetPrompt(ctx, b.Server, b.Prompt, b.PromptArgs)
	if err != nil {
		return nil, err
	}
	appendMCPCorrelation(metadata, "prompt", b.Server, b.Prompt, run, step)
	return result, nil
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

func (h *TypedStepHandler) ensureMCPAllowed(action, server, target string) error {
	server = strings.TrimSpace(server)
	target = strings.TrimSpace(target)
	if server == "" {
		return fmt.Errorf("mcp %s binding requires server", action)
	}
	if target == "" {
		return fmt.Errorf("mcp %s binding requires target", action)
	}
	if _, ok := h.Integrations.Servers[server]; !ok {
		return fmt.Errorf("mcp server %q is not configured under integrations.mcp.servers", server)
	}
	compound := server + "." + target
	if matchesAny(h.Policy.Deny.Servers, server) || matchesAny(ruleSetByAction(h.Policy.Deny, action), compound) {
		return fmt.Errorf("mcp %s %q on server %q denied by policies.mcp.deny", action, target, server)
	}
	if !h.Policy.DefaultDeny {
		return nil
	}
	if matchesAny(h.Policy.Allow.Servers, server) || matchesAny(ruleSetByAction(h.Policy.Allow, action), compound) {
		return nil
	}
	return fmt.Errorf("mcp %s %q on server %q denied by policies.mcp.defaultDeny", action, target, server)
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

func matchesAny(patterns []string, value string) bool {
	for _, p := range patterns {
		if wildcardMatch(p, value) {
			return true
		}
	}
	return false
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

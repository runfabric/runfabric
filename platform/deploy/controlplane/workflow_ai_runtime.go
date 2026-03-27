package controlplane

import (
	"context"
	"fmt"
	"strings"
	"time"

	state "github.com/runfabric/runfabric/platform/core/state/core"
)

// AIStepRunner is an explicit boundary for AI step execution concerns.
type AIStepRunner interface {
	ExecuteStep(ctx context.Context, run *state.WorkflowRun, step state.WorkflowStepRun, output, metadata map[string]any) (*StepExecutionResult, error)
}

// PromptRenderInput describes deterministic prompt composition inputs.
type PromptRenderInput struct {
	BasePrompt string
	MCPPrompt  string
	Step       state.WorkflowStepRun
	Run        *state.WorkflowRun
}

// PromptRenderer builds deterministic prompt text from runtime context.
type PromptRenderer interface {
	Render(in PromptRenderInput) string
}

// DeterministicPromptRenderer composes prompt text in a stable format.
type DeterministicPromptRenderer struct{}

func (DeterministicPromptRenderer) Render(in PromptRenderInput) string {
	parts := []string{}
	if strings.TrimSpace(in.MCPPrompt) != "" {
		parts = append(parts, "MCP Prompt:\n"+strings.TrimSpace(in.MCPPrompt))
	}
	if strings.TrimSpace(in.BasePrompt) != "" {
		parts = append(parts, "Base Prompt:\n"+strings.TrimSpace(in.BasePrompt))
	}
	parts = append(parts, fmt.Sprintf("Step Context:\nstepId=%s\nkind=%s", strings.TrimSpace(in.Step.StepID), strings.TrimSpace(in.Step.Kind)))
	if in.Run != nil {
		parts = append(parts, fmt.Sprintf("Run Context:\nrunId=%s\nworkflowHash=%s", strings.TrimSpace(in.Run.RunID), strings.TrimSpace(in.Run.WorkflowHash)))
	}
	return strings.Join(parts, "\n\n")
}

// DefaultAIStepRunner executes AI step kinds and delegates MCP operations.
type DefaultAIStepRunner struct {
	MCPRuntime     *MCPRuntime
	PromptRenderer PromptRenderer
	// ToolMapper normalizes provider-specific tool call result shapes. Optional.
	ToolMapper ToolResultMapper
	// OutputShaper enriches model output with provider metadata. Optional.
	OutputShaper ModelOutputShaper
	// ModelSelector picks the model for a given step kind and region. Optional.
	ModelSelector ModelSelector
	// RetryStrategy decides whether and how long to retry failed MCP calls. Optional.
	RetryStrategy RetryStrategy
	// CostTracker records per-run token usage costs. Optional.
	CostTracker CostTracker
}

func NewDefaultAIStepRunner(mcpRuntime *MCPRuntime, renderer PromptRenderer) *DefaultAIStepRunner {
	if renderer == nil {
		renderer = DeterministicPromptRenderer{}
	}
	return &DefaultAIStepRunner{MCPRuntime: mcpRuntime, PromptRenderer: renderer}
}

func (r *DefaultAIStepRunner) ExecuteStep(ctx context.Context, run *state.WorkflowRun, step state.WorkflowStepRun, output, metadata map[string]any) (*StepExecutionResult, error) {
	kind := strings.ToLower(strings.TrimSpace(step.Kind))
	region := ""
	provider := ""
	if r.MCPRuntime != nil {
		region = r.MCPRuntime.ActiveRegion
		provider = r.MCPRuntime.Provider
	}
	if r.ModelSelector != nil {
		metadata["selectedModel"] = r.ModelSelector.SelectModel(kind, region)
	}
	var result *StepExecutionResult
	var err error
	switch kind {
	case StepKindAIRetrieval:
		result, err = r.executeAIRetrieval(ctx, run, step, output, metadata)
	case StepKindAIGenerate:
		result, err = r.executeAIGenerate(ctx, run, step, output, metadata)
	case StepKindAIStructured:
		result, err = r.executeAIStructured(step, output, metadata)
	case StepKindAIEval:
		result, err = r.executeAIEval(step, output, metadata)
	default:
		return nil, fmt.Errorf("unsupported ai step kind %q", step.Kind)
	}
	if err == nil && r.CostTracker != nil {
		model, _ := metadata["selectedModel"].(string)
		inputTok := estimateTokens(step.Input)
		outputTok := 10
		if result != nil {
			outputTok = estimateOutputTokens(result.Output)
		}
		r.CostTracker.RecordCost(provider, model, inputTok, outputTok)
	}
	return result, err
}

func (r *DefaultAIStepRunner) executeAIRetrieval(ctx context.Context, run *state.WorkflowRun, step state.WorkflowStepRun, output, metadata map[string]any) (*StepExecutionResult, error) {
	query := strings.TrimSpace(asInputString(step.Input, "query"))
	if query == "" {
		return nil, fmt.Errorf("step %s kind ai-retrieval requires input.query", step.StepID)
	}
	documents := []any{}
	if binding, ok := ParseMCPBinding(step.Input); ok {
		if binding.Resource != "" {
			result, err := r.readResourceWithRetry(ctx, run, step, binding, metadata)
			if err != nil {
				return nil, err
			}
			if r.ToolMapper != nil {
				result = r.ToolMapper.MapToolResult(binding.Server, result)
			}
			documents = append(documents, result)
		}
		if binding.Tool != "" {
			result, err := r.callToolWithRetry(ctx, run, step, binding, metadata)
			if err != nil {
				return nil, err
			}
			if r.ToolMapper != nil {
				result = r.ToolMapper.MapToolResult(binding.Server, result)
			}
			documents = append(documents, result)
		}
	}
	output["query"] = query
	output["documents"] = documents
	rawOut := map[string]any{"type": "retrieval", "documents": documents}
	if r.OutputShaper != nil {
		rawOut = r.OutputShaper.ShapeOutput(step.Kind, step.StepID, rawOut)
	}
	output["modelOutput"] = rawOut
	return &StepExecutionResult{Output: output, Metadata: metadata}, nil
}

func (r *DefaultAIStepRunner) executeAIGenerate(ctx context.Context, run *state.WorkflowRun, step state.WorkflowStepRun, output, metadata map[string]any) (*StepExecutionResult, error) {
	prompt := strings.TrimSpace(asInputString(step.Input, "prompt"))
	if prompt == "" {
		return nil, fmt.Errorf("step %s kind ai-generate requires input.prompt", step.StepID)
	}
	var mcpPromptText string
	if binding, ok := ParseMCPBinding(step.Input); ok {
		if binding.Prompt != "" {
			result, err := r.getPromptWithRetry(ctx, run, step, binding, metadata)
			if err != nil {
				return nil, err
			}
			if text, ok := result["text"].(string); ok {
				mcpPromptText = text
			}
			output["mcpPrompt"] = result
		}
		if binding.Tool != "" {
			toolRes, err := r.callToolWithRetry(ctx, run, step, binding, metadata)
			if err != nil {
				return nil, err
			}
			if r.ToolMapper != nil {
				toolRes = r.ToolMapper.MapToolResult(binding.Server, toolRes)
			}
			output["mcpTool"] = toolRes
			output["toolResults"] = []any{toolRes}
		}
	}
	renderedPrompt := r.PromptRenderer.Render(PromptRenderInput{
		BasePrompt: prompt,
		MCPPrompt:  mcpPromptText,
		Step:       step,
		Run:        run,
	})
	text := fmt.Sprintf("generated(%s): %s", step.StepID, renderedPrompt)
	output["text"] = text
	rawOut := map[string]any{"type": "text", "text": text}
	if r.OutputShaper != nil {
		rawOut = r.OutputShaper.ShapeOutput(step.Kind, step.StepID, rawOut)
	}
	output["modelOutput"] = rawOut
	return &StepExecutionResult{Output: output, Metadata: metadata}, nil
}

func (r *DefaultAIStepRunner) executeAIStructured(step state.WorkflowStepRun, output, metadata map[string]any) (*StepExecutionResult, error) {
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
	output["modelOutput"] = map[string]any{"type": "object", "object": obj}
	return &StepExecutionResult{Output: output, Metadata: metadata}, nil
}

func (r *DefaultAIStepRunner) executeAIEval(step state.WorkflowStepRun, output, metadata map[string]any) (*StepExecutionResult, error) {
	score, ok := asFloat(step.Input["score"])
	if !ok {
		return nil, fmt.Errorf("step %s kind ai-eval requires numeric input.score", step.StepID)
	}
	threshold := 0.5
	if v, ok := asFloat(step.Input["threshold"]); ok {
		threshold = v
	}
	pass := score >= threshold
	output["score"] = score
	output["threshold"] = threshold
	output["pass"] = pass
	output["modelOutput"] = map[string]any{"type": "eval", "pass": pass, "score": score, "threshold": threshold}
	return &StepExecutionResult{Output: output, Metadata: metadata}, nil
}

func (r *DefaultAIStepRunner) callToolWithRetry(ctx context.Context, run *state.WorkflowRun, step state.WorkflowStepRun, b MCPBinding, metadata map[string]any) (map[string]any, error) {
	for attempt := 1; ; attempt++ {
		result, err := r.MCPRuntime.CallTool(ctx, run, step, b, metadata)
		if err == nil || r.RetryStrategy == nil || !r.RetryStrategy.ShouldRetry(attempt, err) {
			return result, err
		}
		time.Sleep(r.RetryStrategy.Backoff(attempt))
	}
}

func (r *DefaultAIStepRunner) readResourceWithRetry(ctx context.Context, run *state.WorkflowRun, step state.WorkflowStepRun, b MCPBinding, metadata map[string]any) (map[string]any, error) {
	for attempt := 1; ; attempt++ {
		result, err := r.MCPRuntime.ReadResource(ctx, run, step, b, metadata)
		if err == nil || r.RetryStrategy == nil || !r.RetryStrategy.ShouldRetry(attempt, err) {
			return result, err
		}
		time.Sleep(r.RetryStrategy.Backoff(attempt))
	}
}

func (r *DefaultAIStepRunner) getPromptWithRetry(ctx context.Context, run *state.WorkflowRun, step state.WorkflowStepRun, b MCPBinding, metadata map[string]any) (map[string]any, error) {
	for attempt := 1; ; attempt++ {
		result, err := r.MCPRuntime.GetPrompt(ctx, run, step, b, metadata)
		if err == nil || r.RetryStrategy == nil || !r.RetryStrategy.ShouldRetry(attempt, err) {
			return result, err
		}
		time.Sleep(r.RetryStrategy.Backoff(attempt))
	}
}

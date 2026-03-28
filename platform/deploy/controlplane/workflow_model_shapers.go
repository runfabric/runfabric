package controlplane

import "strings"

// ModelOutputShaper enriches raw model output maps with provider-specific metadata
// such as usage statistics, model identifiers, and finish reasons.
// It is called on the modelOutput envelope after each AI step executes.
type ModelOutputShaper interface {
	ShapeOutput(kind string, stepID string, raw map[string]any) map[string]any
}

// DefaultModelOutputShaper returns the raw output unchanged. Used as the default.
type DefaultModelOutputShaper struct{}

func (DefaultModelOutputShaper) ShapeOutput(_, _ string, raw map[string]any) map[string]any {
	return raw
}

// AWSBedrockOutputShaper enriches model output with AWS Bedrock Converse API usage metadata.
// Adds usage.inputTokens, usage.outputTokens, the model ID, and a stop reason.
type AWSBedrockOutputShaper struct{}

func (AWSBedrockOutputShaper) ShapeOutput(kind, stepID string, raw map[string]any) map[string]any {
	out := copyMap(raw)
	out["provider"] = "aws"
	out["model"] = resolvedModelForOutput(kind, raw, bedrockModelForKind)
	if _, ok := out["usage"]; !ok {
		out["usage"] = map[string]any{
			"inputTokens":  estimateTokens(raw),
			"outputTokens": estimateOutputTokens(raw),
		}
	}
	out["stopReason"] = "end_turn"
	return out
}

// GCPVertexOutputShaper enriches model output with GCP Vertex AI (Gemini) usage metadata.
// Adds usageMetadata.promptTokenCount, candidatesTokenCount, totalTokenCount, and a finish reason.
type GCPVertexOutputShaper struct{}

func (GCPVertexOutputShaper) ShapeOutput(kind, stepID string, raw map[string]any) map[string]any {
	out := copyMap(raw)
	out["provider"] = "gcp"
	out["model"] = resolvedModelForOutput(kind, raw, vertexModelForKind)
	if _, ok := out["usageMetadata"]; !ok {
		inTok := estimateTokens(raw)
		outTok := estimateOutputTokens(raw)
		out["usageMetadata"] = map[string]any{
			"promptTokenCount":     inTok,
			"candidatesTokenCount": outTok,
			"totalTokenCount":      inTok + outTok,
		}
	}
	out["finishReason"] = "STOP"
	return out
}

// AzureOpenAIOutputShaper enriches model output with Azure OpenAI chat completion metadata.
// Adds usage.prompt_tokens, usage.completion_tokens, usage.total_tokens, and a finish reason.
type AzureOpenAIOutputShaper struct{}

func (AzureOpenAIOutputShaper) ShapeOutput(kind, stepID string, raw map[string]any) map[string]any {
	out := copyMap(raw)
	out["provider"] = "azure"
	out["model"] = resolvedModelForOutput(kind, raw, azureModelForKind)
	if _, ok := out["usage"]; !ok {
		inTok := estimateTokens(raw)
		outTok := estimateOutputTokens(raw)
		out["usage"] = map[string]any{
			"prompt_tokens":     inTok,
			"completion_tokens": outTok,
			"total_tokens":      inTok + outTok,
		}
	}
	out["finish_reason"] = "stop"
	return out
}

// ProviderModelOutputShaper returns the appropriate ModelOutputShaper for a cloud provider.
// Falls back to DefaultModelOutputShaper for unknown providers.
func ProviderModelOutputShaper(provider string) ModelOutputShaper {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "aws":
		return AWSBedrockOutputShaper{}
	case "gcp":
		return GCPVertexOutputShaper{}
	case "azure":
		return AzureOpenAIOutputShaper{}
	default:
		return DefaultModelOutputShaper{}
	}
}

func bedrockModelForKind(kind string) string {
	switch kind {
	case StepKindAIStructured, StepKindAIEval:
		return "anthropic.claude-3-haiku-20240307-v1:0"
	default:
		return "anthropic.claude-3-sonnet-20240229-v1:0"
	}
}

func vertexModelForKind(kind string) string {
	switch kind {
	case StepKindAIEval:
		return "gemini-1.5-flash-001"
	default:
		return "gemini-1.5-pro-001"
	}
}

func azureModelForKind(kind string) string {
	switch kind {
	case StepKindAIEval:
		return "gpt-4o-mini"
	default:
		return "gpt-4o"
	}
}

func resolvedModelForOutput(kind string, raw map[string]any, fallback func(string) string) string {
	if raw != nil {
		if model, ok := raw["model"].(string); ok && strings.TrimSpace(model) != "" {
			return strings.TrimSpace(model)
		}
	}
	return fallback(kind)
}

// estimateTokens returns a crude token estimate (4 chars ≈ 1 token) from string values in a map.
func estimateTokens(raw map[string]any) int {
	total := 0
	for _, v := range raw {
		if s, ok := v.(string); ok {
			total += len(s) / 4
		}
	}
	if total < 1 {
		return 1
	}
	return total
}

// estimateOutputTokens estimates output tokens from the "text" field if present.
func estimateOutputTokens(raw map[string]any) int {
	if t, ok := raw["text"].(string); ok {
		n := len(t) / 4
		if n < 1 {
			return 1
		}
		return n
	}
	return 10
}

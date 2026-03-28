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
	out["provider"] = "aws-lambda"
	out["model"] = resolvedModelForOutput(raw, genericModelFallback)
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
	out["provider"] = "gcp-functions"
	out["model"] = resolvedModelForOutput(raw, genericModelFallback)
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
	out["provider"] = "azure-functions"
	out["model"] = resolvedModelForOutput(raw, genericModelFallback)
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

// ProviderModelOutputShaper returns the appropriate ModelOutputShaper for a provider id.
// Falls back to DefaultModelOutputShaper for unknown providers.
func ProviderModelOutputShaper(provider string) ModelOutputShaper {
	if policy, ok := providerModelPolicyFor(provider); ok && policy.shaper != nil {
		return policy.shaper
	}
	return DefaultModelOutputShaper{}
}

func resolvedModelForOutput(raw map[string]any, fallback string) string {
	if raw != nil {
		if model, ok := raw["model"].(string); ok && strings.TrimSpace(model) != "" {
			return strings.TrimSpace(model)
		}
	}
	return fallback
}

const genericModelFallback = "model-default"

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

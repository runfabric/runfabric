package controlplane

import (
	"os"
	"strings"
)

// ModelSelector picks the appropriate model identifier for a given AI step kind
// based on provider region availability and step requirements.
type ModelSelector interface {
	// SelectModel returns a model identifier string for the given step kind and region.
	SelectModel(kind string, region string) string
}

// ConfigurableModelSelector applies config-based overrides on top of a base selector.
// Override keys are step kinds (ai-retrieval, ai-generate, ai-structured, ai-eval) and "default".
type ConfigurableModelSelector struct {
	Base      ModelSelector
	Overrides map[string]string
}

func (s ConfigurableModelSelector) SelectModel(kind, region string) string {
	normalizedKind := strings.ToLower(strings.TrimSpace(kind))
	if model := strings.TrimSpace(s.Overrides[normalizedKind]); model != "" {
		return model
	}
	if model := strings.TrimSpace(s.Overrides["default"]); model != "" {
		return model
	}
	if s.Base == nil {
		return ""
	}
	return s.Base.SelectModel(kind, region)
}

func WithModelSelectorOverrides(base ModelSelector, overrides map[string]string) ModelSelector {
	if len(overrides) == 0 {
		return base
	}
	return ConfigurableModelSelector{
		Base:      base,
		Overrides: overrides,
	}
}

func EnvModelSelectorOverrides(provider string) map[string]string {
	overrides := map[string]string{}
	// Global overrides apply to all providers.
	collectEnvModelOverride(overrides, "default", "RUNFABRIC_MODEL_DEFAULT")
	collectEnvModelOverride(overrides, StepKindAIRetrieval, "RUNFABRIC_MODEL_AI_RETRIEVAL")
	collectEnvModelOverride(overrides, StepKindAIGenerate, "RUNFABRIC_MODEL_AI_GENERATE")
	collectEnvModelOverride(overrides, StepKindAIStructured, "RUNFABRIC_MODEL_AI_STRUCTURED")
	collectEnvModelOverride(overrides, StepKindAIEval, "RUNFABRIC_MODEL_AI_EVAL")

	// Provider-scoped env keys override globals for that provider.
	p := strings.ToUpper(strings.TrimSpace(provider))
	if p != "" {
		collectEnvModelOverride(overrides, "default", "RUNFABRIC_MODEL_"+p+"_DEFAULT")
		collectEnvModelOverride(overrides, StepKindAIRetrieval, "RUNFABRIC_MODEL_"+p+"_AI_RETRIEVAL")
		collectEnvModelOverride(overrides, StepKindAIGenerate, "RUNFABRIC_MODEL_"+p+"_AI_GENERATE")
		collectEnvModelOverride(overrides, StepKindAIStructured, "RUNFABRIC_MODEL_"+p+"_AI_STRUCTURED")
		collectEnvModelOverride(overrides, StepKindAIEval, "RUNFABRIC_MODEL_"+p+"_AI_EVAL")
	}
	return overrides
}

func collectEnvModelOverride(overrides map[string]string, kind, envKey string) {
	if model := strings.TrimSpace(os.Getenv(envKey)); model != "" {
		overrides[kind] = model
	}
}

// DefaultModelSelector provides concrete fallback model IDs for non-cloud/unknown providers.
type DefaultModelSelector struct{}

func (DefaultModelSelector) SelectModel(kind, _ string) string {
	switch kind {
	case StepKindAIEval, StepKindAIRetrieval:
		return "gpt-4o-mini"
	default:
		return "gpt-4o"
	}
}

// AWSModelSelector routes to Bedrock model families based on region availability.
// Version pinning is intentionally avoided here; prefer config/env overrides for exact IDs.
type AWSModelSelector struct{}

// awsSonnetRegions lists AWS regions with stronger model family availability.
var awsSonnetRegions = map[string]bool{
	"us-east-1":      true,
	"us-west-2":      true,
	"eu-west-1":      true,
	"ap-northeast-1": true,
}

func (AWSModelSelector) SelectModel(kind, region string) string {
	switch kind {
	case StepKindAIEval, StepKindAIStructured:
		return "anthropic.claude-3-haiku"
	default:
		if awsSonnetRegions[strings.ToLower(strings.TrimSpace(region))] {
			return "anthropic.claude-3-sonnet"
		}
		return "anthropic.claude-3-haiku"
	}
}

// GCPModelSelector routes to Vertex AI model families by step use-case.
// Version pinning is intentionally avoided here; prefer config/env overrides for exact IDs.
type GCPModelSelector struct{}

func (GCPModelSelector) SelectModel(kind, _ string) string {
	switch kind {
	case StepKindAIEval, StepKindAIRetrieval:
		return "gemini-flash"
	default:
		return "gemini-pro"
	}
}

// AzureModelSelector picks Azure OpenAI deployment names by step kind.
// Operators can construct a custom AzureModelSelector to override deployment names.
type AzureModelSelector struct{}

func (AzureModelSelector) SelectModel(kind, _ string) string {
	switch kind {
	case StepKindAIEval, StepKindAIRetrieval:
		return "gpt-4o-mini"
	default:
		return "gpt-4o"
	}
}

// ProviderModelSelector returns the appropriate ModelSelector for a cloud provider.
// Falls back to DefaultModelSelector for unknown providers.
func ProviderModelSelector(provider string) ModelSelector {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "aws":
		return AWSModelSelector{}
	case "gcp":
		return GCPModelSelector{}
	case "azure":
		return AzureModelSelector{}
	default:
		return DefaultModelSelector{}
	}
}

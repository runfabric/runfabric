package controlplane

import "strings"

// ModelSelector picks the appropriate model identifier for a given AI step kind
// based on provider region availability and step requirements.
type ModelSelector interface {
	// SelectModel returns a model identifier string for the given step kind and region.
	SelectModel(kind string, region string) string
}

// DefaultModelSelector returns a generic model placeholder for non-cloud environments.
type DefaultModelSelector struct{}

func (DefaultModelSelector) SelectModel(_, _ string) string {
	return "default-model"
}

// AWSModelSelector routes to Bedrock model IDs based on region availability.
// Claude 3 Sonnet is preferred in regions with full Bedrock support; Haiku elsewhere.
type AWSModelSelector struct{}

// awsSonnetRegions lists AWS regions with Claude 3 Sonnet availability on Bedrock.
var awsSonnetRegions = map[string]bool{
	"us-east-1":      true,
	"us-west-2":      true,
	"eu-west-1":      true,
	"ap-northeast-1": true,
}

func (AWSModelSelector) SelectModel(kind, region string) string {
	switch kind {
	case StepKindAIEval, StepKindAIStructured:
		// Cost-optimized: use Haiku for classification and eval steps.
		return "anthropic.claude-3-haiku-20240307-v1:0"
	default:
		if awsSonnetRegions[strings.ToLower(strings.TrimSpace(region))] {
			return "anthropic.claude-3-sonnet-20240229-v1:0"
		}
		return "anthropic.claude-3-haiku-20240307-v1:0"
	}
}

// GCPModelSelector routes to Vertex AI model IDs (Gemini family) by step use-case.
// Gemini 1.5 Pro for generation and structured extraction; Flash for eval/retrieval.
type GCPModelSelector struct{}

func (GCPModelSelector) SelectModel(kind, _ string) string {
	switch kind {
	case StepKindAIEval, StepKindAIRetrieval:
		return "gemini-1.5-flash-001"
	default:
		return "gemini-1.5-pro-001"
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

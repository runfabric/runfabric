package runtime

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// LLMClient makes text generation calls to a cloud LLM API.
type LLMClient interface {
	Generate(ctx context.Context, model, prompt string) (string, error)
	GenerateJSON(ctx context.Context, model, prompt string, schema map[string]any) (map[string]any, error)
	Evaluate(ctx context.Context, model, content, criteria string) (float64, error)
}

// LLMClientFactory constructs an LLMClient for a given region.
// The factory is responsible for reading provider-specific env vars.
// Return nil when required configuration is absent.
type LLMClientFactory func(region string) LLMClient

var llmRegistry = map[string]LLMClientFactory{}

// RegisterLLMClient registers a factory for a provider ID (e.g. "aws-lambda").
// Call from init() in provider-specific files or from external extension packages.
func RegisterLLMClient(providerID string, factory LLMClientFactory) {
	llmRegistry[normalizedProviderID(providerID)] = factory
}

// ProviderLLMClient returns an LLMClient for the given provider using env-var configuration.
// Returns nil if required configuration is absent — callers must check for nil.
func ProviderLLMClient(provider, region string) LLMClient {
	id := normalizedProviderID(provider)
	if factory, ok := llmRegistry[id]; ok {
		return factory(region)
	}
	// Fallback: generic OpenAI-compatible endpoint from env.
	endpoint := os.Getenv("RUNFABRIC_LLM_ENDPOINT")
	apiKey := os.Getenv("RUNFABRIC_LLM_API_KEY")
	if endpoint == "" {
		return nil
	}
	return NewHTTPLLMClient(endpoint, apiKey)
}

var evalScorePattern = regexp.MustCompile(`\d+\.?\d*`)

func parseEvalScore(text string) (float64, error) {
	// First try exact parse, then fall back to extracting the first number in the string.
	if score, err := strconv.ParseFloat(strings.TrimSpace(text), 64); err == nil {
		return clampScore(score), nil
	}
	match := evalScorePattern.FindString(text)
	if match == "" {
		return 0, fmt.Errorf("llm evaluation returned non-numeric score %q", text)
	}
	score, err := strconv.ParseFloat(match, 64)
	if err != nil {
		return 0, fmt.Errorf("llm evaluation returned non-numeric score %q: %w", text, err)
	}
	return clampScore(score), nil
}

func clampScore(score float64) float64 {
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

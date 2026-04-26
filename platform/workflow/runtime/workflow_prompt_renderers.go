package runtime

import (
	"fmt"
	"strings"
)

// AWSBedrockPromptRenderer formats prompts for AWS Bedrock Converse API.
// It structures context as a [system] block followed by the [user] turn, matching
// the multi-turn message format used by Claude models on Bedrock.
type AWSBedrockPromptRenderer struct{}

func (AWSBedrockPromptRenderer) Render(in PromptRenderInput) string {
	var system strings.Builder
	system.WriteString("[provider:bedrock model:claude]")
	if in.Run != nil {
		system.WriteString(fmt.Sprintf(" run:%s workflow:%s", in.Run.RunID, in.Run.WorkflowHash))
	}
	system.WriteString(fmt.Sprintf(" step:%s kind:%s", in.Step.StepID, in.Step.Kind))

	var user strings.Builder
	if strings.TrimSpace(in.MCPPrompt) != "" {
		user.WriteString("Context:\n")
		user.WriteString(strings.TrimSpace(in.MCPPrompt))
		user.WriteString("\n\n")
	}
	if strings.TrimSpace(in.BasePrompt) != "" {
		user.WriteString(strings.TrimSpace(in.BasePrompt))
	}
	return fmt.Sprintf("[system]\n%s\n[user]\n%s", system.String(), strings.TrimSpace(user.String()))
}

// GCPVertexPromptRenderer formats prompts for GCP Vertex AI (Gemini) generateContent API.
// It uses a parts-based structure matching Gemini's content role format.
type GCPVertexPromptRenderer struct{}

func (GCPVertexPromptRenderer) Render(in PromptRenderInput) string {
	var parts []string
	if in.Run != nil {
		parts = append(parts, fmt.Sprintf("[provider:vertex model:gemini] run:%s step:%s kind:%s",
			in.Run.RunID, in.Step.StepID, in.Step.Kind))
	} else {
		parts = append(parts, fmt.Sprintf("[provider:vertex model:gemini] step:%s kind:%s",
			in.Step.StepID, in.Step.Kind))
	}
	if strings.TrimSpace(in.MCPPrompt) != "" {
		parts = append(parts, "[context]\n"+strings.TrimSpace(in.MCPPrompt))
	}
	if strings.TrimSpace(in.BasePrompt) != "" {
		parts = append(parts, "[instruction]\n"+strings.TrimSpace(in.BasePrompt))
	}
	return strings.Join(parts, "\n\n")
}

// AzureOpenAIPromptRenderer formats prompts for Azure OpenAI chat completion API.
// It produces a system+user message structure matching the GPT-4 chat format.
type AzureOpenAIPromptRenderer struct{}

func (AzureOpenAIPromptRenderer) Render(in PromptRenderInput) string {
	var system strings.Builder
	system.WriteString("[provider:azure-openai model:gpt-4o]")
	if in.Run != nil {
		system.WriteString(fmt.Sprintf(" run:%s workflow:%s", in.Run.RunID, in.Run.WorkflowHash))
	}

	var content strings.Builder
	if strings.TrimSpace(in.MCPPrompt) != "" {
		content.WriteString("Additional context:\n")
		content.WriteString(strings.TrimSpace(in.MCPPrompt))
		content.WriteString("\n\n")
	}
	if strings.TrimSpace(in.BasePrompt) != "" {
		content.WriteString(strings.TrimSpace(in.BasePrompt))
	}
	content.WriteString(fmt.Sprintf("\n\n[step:%s kind:%s]", in.Step.StepID, in.Step.Kind))
	return fmt.Sprintf("[system] %s\n[user] %s", system.String(), strings.TrimSpace(content.String()))
}

// ProviderPromptRenderer returns the appropriate PromptRenderer for a given provider name.
// Falls back to DeterministicPromptRenderer for unknown or empty providers.
func ProviderPromptRenderer(provider string) PromptRenderer {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "aws-lambda":
		return AWSBedrockPromptRenderer{}
	case "gcp-functions":
		return GCPVertexPromptRenderer{}
	case "azure-functions":
		return AzureOpenAIPromptRenderer{}
	default:
		return DeterministicPromptRenderer{}
	}
}

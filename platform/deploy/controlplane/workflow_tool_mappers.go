package controlplane

import "strings"

// ToolResultMapper normalizes provider-specific MCP tool call results to the RunFabric
// standard format: {"server": string, "name": string, "result": any, "provider": string}.
type ToolResultMapper interface {
	MapToolResult(server string, raw map[string]any) map[string]any
}

// NoopToolResultMapper returns the raw tool result unchanged. Used as the default.
type NoopToolResultMapper struct{}

func (NoopToolResultMapper) MapToolResult(_ string, raw map[string]any) map[string]any {
	return raw
}

// AWSToolResultMapper normalizes AWS Bedrock Converse API tool result shapes.
// Bedrock wraps results as: {"toolResult": {"content": [{"text": "..."}]}}
type AWSToolResultMapper struct{}

func (AWSToolResultMapper) MapToolResult(server string, raw map[string]any) map[string]any {
	out := copyMap(raw)
	// Unwrap Bedrock-style nested content array.
	if tr, ok := raw["toolResult"].(map[string]any); ok {
		if content, ok := tr["content"].([]any); ok && len(content) > 0 {
			if first, ok := content[0].(map[string]any); ok {
				if text, ok := first["text"].(string); ok {
					out["result"] = text
				}
			}
		}
	}
	out["provider"] = "aws-lambda"
	return out
}

// GCPToolResultMapper normalizes GCP Vertex AI functionResponse shapes.
// Vertex AI returns: {"functionResponse": {"name": "...", "response": {...}}}
type GCPToolResultMapper struct{}

func (GCPToolResultMapper) MapToolResult(server string, raw map[string]any) map[string]any {
	out := copyMap(raw)
	if fr, ok := raw["functionResponse"].(map[string]any); ok {
		if resp, ok := fr["response"]; ok {
			out["result"] = resp
		}
		if name, ok := fr["name"].(string); ok && out["name"] == nil {
			out["name"] = name
		}
	}
	out["provider"] = "gcp-functions"
	return out
}

// AzureToolResultMapper normalizes Azure OpenAI tool call result shapes.
// Azure OpenAI returns results as: {"role": "tool", "content": "..."}
type AzureToolResultMapper struct{}

func (AzureToolResultMapper) MapToolResult(server string, raw map[string]any) map[string]any {
	out := copyMap(raw)
	if content, ok := raw["content"].(string); ok {
		out["result"] = content
	}
	out["provider"] = "azure-functions"
	return out
}

// ProviderToolResultMapper returns the appropriate ToolResultMapper for a cloud provider.
// Falls back to NoopToolResultMapper for unknown providers.
func ProviderToolResultMapper(provider string) ToolResultMapper {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "aws-lambda":
		return AWSToolResultMapper{}
	case "gcp-functions":
		return GCPToolResultMapper{}
	case "azure-functions":
		return AzureToolResultMapper{}
	default:
		return NoopToolResultMapper{}
	}
}

// copyMap returns a shallow copy of m.
func copyMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

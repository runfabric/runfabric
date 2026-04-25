package controlplane

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func init() {
	RegisterLLMClient("azure-functions", func(region string) LLMClient {
		endpoint := os.Getenv("RUNFABRIC_AZURE_OPENAI_ENDPOINT")
		apiKey := os.Getenv("RUNFABRIC_AZURE_OPENAI_API_KEY")
		if endpoint == "" || apiKey == "" {
			return nil
		}
		return NewHTTPLLMClient(endpoint, apiKey)
	})
}

// HTTPLLMClient calls an OpenAI-compatible chat completion API.
type HTTPLLMClient struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

func NewHTTPLLMClient(baseURL, apiKey string) *HTTPLLMClient {
	return &HTTPLLMClient{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		APIKey:     apiKey,
		HTTPClient: &http.Client{Timeout: 120 * time.Second},
	}
}

type chatCompletionRequest struct {
	Model          string          `json:"model"`
	Messages       []chatMessage   `json:"messages"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseFormat struct {
	Type       string      `json:"type"`
	JSONSchema *jsonSchema `json:"json_schema,omitempty"`
}

type jsonSchema struct {
	Name   string         `json:"name"`
	Schema map[string]any `json:"schema"`
	Strict bool           `json:"strict"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (c *HTTPLLMClient) doCompletion(ctx context.Context, req chatCompletionRequest) (string, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal llm request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build llm request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("llm request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read llm response body: %w", err)
	}
	var result chatCompletionResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("decode llm response (status %d): %w", resp.StatusCode, err)
	}
	if result.Error != nil {
		return "", fmt.Errorf("llm api error: %s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("llm returned no choices (status %d)", resp.StatusCode)
	}
	return strings.TrimSpace(result.Choices[0].Message.Content), nil
}

func (c *HTTPLLMClient) Generate(ctx context.Context, model, prompt string) (string, error) {
	return c.doCompletion(ctx, chatCompletionRequest{
		Model:    model,
		Messages: []chatMessage{{Role: "user", Content: prompt}},
	})
}

func (c *HTTPLLMClient) GenerateJSON(ctx context.Context, model, prompt string, schema map[string]any) (map[string]any, error) {
	req := chatCompletionRequest{
		Model:    model,
		Messages: []chatMessage{{Role: "user", Content: prompt}},
		ResponseFormat: &responseFormat{
			Type: "json_schema",
			JSONSchema: &jsonSchema{
				Name:   "structured_output",
				Schema: schema,
				Strict: true,
			},
		},
	}
	text, err := c.doCompletion(ctx, req)
	if err != nil {
		return nil, err
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(text), &obj); err != nil {
		return nil, fmt.Errorf("llm returned invalid JSON for structured output: %w", err)
	}
	return obj, nil
}

func (c *HTTPLLMClient) Evaluate(ctx context.Context, model, content, criteria string) (float64, error) {
	evalPrompt := fmt.Sprintf(
		"Evaluate the following content against the given criteria. Reply with ONLY a numeric score from 0.0 to 1.0 and nothing else.\n\nCriteria: %s\n\nContent:\n%s",
		criteria, content,
	)
	text, err := c.doCompletion(ctx, chatCompletionRequest{
		Model:     model,
		Messages:  []chatMessage{{Role: "user", Content: evalPrompt}},
		MaxTokens: 10,
	})
	if err != nil {
		return 0, err
	}
	return parseEvalScore(text)
}

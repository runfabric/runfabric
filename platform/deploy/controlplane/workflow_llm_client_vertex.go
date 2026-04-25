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
	RegisterLLMClient("gcp-functions", func(region string) LLMClient {
		project := os.Getenv("RUNFABRIC_GCP_PROJECT_ID")
		if project == "" {
			project = os.Getenv("GOOGLE_CLOUD_PROJECT")
		}
		r := strings.TrimSpace(region)
		if r == "" {
			r = os.Getenv("RUNFABRIC_GCP_REGION")
		}
		if project == "" || r == "" {
			return nil
		}
		return NewVertexLLMClient(project, r)
	})
}

// VertexTokenSource provides OAuth2 access tokens for Vertex AI.
// Implement this interface to enable automatic token refresh (e.g. via ADC or
// golang.org/x/oauth2/google). When nil, VertexLLMClient falls back to the
// RUNFABRIC_GCP_ACCESS_TOKEN / GOOGLE_ACCESS_TOKEN environment variables.
type VertexTokenSource interface {
	Token() (string, error)
}

// VertexLLMClient calls the GCP Vertex AI generateContent API.
// Set TokenSource to enable automatic token refresh; otherwise env vars are used.
type VertexLLMClient struct {
	ProjectID   string
	Region      string
	HTTPClient  *http.Client
	TokenSource VertexTokenSource
}

func NewVertexLLMClient(projectID, region string) *VertexLLMClient {
	return &VertexLLMClient{
		ProjectID:  projectID,
		Region:     region,
		HTTPClient: &http.Client{Timeout: 120 * time.Second},
	}
}

type vertexGenerateRequest struct {
	Contents         []vertexContent  `json:"contents"`
	GenerationConfig *vertexGenConfig `json:"generationConfig,omitempty"`
}

type vertexGenConfig struct {
	ResponseMIMEType string         `json:"responseMimeType,omitempty"`
	ResponseSchema   map[string]any `json:"responseSchema,omitempty"`
}

type vertexContent struct {
	Role  string       `json:"role"`
	Parts []vertexPart `json:"parts"`
}

type vertexPart struct {
	Text string `json:"text"`
}

type vertexGenerateResponse struct {
	Candidates []struct {
		Content vertexContent `json:"content"`
	} `json:"candidates"`
}

func (c *VertexLLMClient) accessToken() (string, error) {
	if c.TokenSource != nil {
		token, err := c.TokenSource.Token()
		if err != nil {
			return "", fmt.Errorf("get vertex token: %w", err)
		}
		return token, nil
	}
	token := strings.TrimSpace(os.Getenv("RUNFABRIC_GCP_ACCESS_TOKEN"))
	if token == "" {
		token = strings.TrimSpace(os.Getenv("GOOGLE_ACCESS_TOKEN"))
	}
	if token == "" {
		return "", fmt.Errorf("RUNFABRIC_GCP_ACCESS_TOKEN or GOOGLE_ACCESS_TOKEN must be set for Vertex AI (or configure Application Default Credentials)")
	}
	return token, nil
}

func (c *VertexLLMClient) post(ctx context.Context, model string, body []byte) ([]byte, error) {
	url := fmt.Sprintf(
		"https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:generateContent",
		c.Region, c.ProjectID, c.Region, model,
	)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	token, err := c.accessToken()
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("vertex request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read vertex response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vertex returned %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	return raw, nil
}

func (c *VertexLLMClient) generate(ctx context.Context, model, prompt string) (string, error) {
	req := vertexGenerateRequest{
		Contents: []vertexContent{
			{Role: "user", Parts: []vertexPart{{Text: prompt}}},
		},
	}
	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal vertex request: %w", err)
	}
	raw, err := c.post(ctx, model, body)
	if err != nil {
		return "", err
	}
	var result vertexGenerateResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("decode vertex response: %w", err)
	}
	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("vertex returned empty candidates")
	}
	return strings.TrimSpace(result.Candidates[0].Content.Parts[0].Text), nil
}

func (c *VertexLLMClient) Generate(ctx context.Context, model, prompt string) (string, error) {
	return c.generate(ctx, model, prompt)
}

// GenerateJSON uses the Vertex AI responseSchema generationConfig for native structured output.
func (c *VertexLLMClient) GenerateJSON(ctx context.Context, model, prompt string, schema map[string]any) (map[string]any, error) {
	req := vertexGenerateRequest{
		Contents: []vertexContent{
			{Role: "user", Parts: []vertexPart{{Text: prompt}}},
		},
		GenerationConfig: &vertexGenConfig{
			ResponseMIMEType: "application/json",
			ResponseSchema:   schema,
		},
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal vertex request: %w", err)
	}
	raw, err := c.post(ctx, model, body)
	if err != nil {
		return nil, err
	}
	var result vertexGenerateResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decode vertex response: %w", err)
	}
	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("vertex returned empty candidates for structured output")
	}
	text := strings.TrimSpace(result.Candidates[0].Content.Parts[0].Text)
	var obj map[string]any
	if err := json.Unmarshal([]byte(text), &obj); err != nil {
		return nil, fmt.Errorf("vertex returned invalid JSON for structured output: %w", err)
	}
	return obj, nil
}

func (c *VertexLLMClient) Evaluate(ctx context.Context, model, content, criteria string) (float64, error) {
	evalPrompt := fmt.Sprintf(
		"Evaluate the following content against the given criteria. Reply with ONLY a numeric score from 0.0 to 1.0.\n\nCriteria: %s\n\nContent:\n%s",
		criteria, content,
	)
	text, err := c.generate(ctx, model, evalPrompt)
	if err != nil {
		return 0, err
	}
	return parseEvalScore(text)
}

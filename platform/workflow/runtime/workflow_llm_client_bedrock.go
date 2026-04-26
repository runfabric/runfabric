package runtime

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func init() {
	RegisterLLMClient("aws-lambda", func(region string) LLMClient {
		r := strings.TrimSpace(region)
		if r == "" {
			r = os.Getenv("AWS_REGION")
		}
		if r == "" {
			r = os.Getenv("AWS_DEFAULT_REGION")
		}
		if r == "" {
			return nil
		}
		return NewBedrockLLMClient(r)
	})
}

// BedrockLLMClient calls the AWS Bedrock Converse API with SigV4 signing.
type BedrockLLMClient struct {
	Region     string
	HTTPClient *http.Client
}

func NewBedrockLLMClient(region string) *BedrockLLMClient {
	return &BedrockLLMClient{
		Region:     region,
		HTTPClient: &http.Client{Timeout: 120 * time.Second},
	}
}

type bedrockConverseRequest struct {
	Messages []bedrockMessage `json:"messages"`
}

type bedrockMessage struct {
	Role    string                `json:"role"`
	Content []bedrockContentBlock `json:"content"`
}

type bedrockContentBlock struct {
	Text string `json:"text"`
}

type bedrockConverseResponse struct {
	Output struct {
		Message bedrockMessage `json:"message"`
	} `json:"output"`
}

// bedrockToolUseRequest uses the Converse toolConfig to force structured JSON output.
type bedrockToolUseRequest struct {
	Messages   []bedrockMessage  `json:"messages"`
	ToolConfig bedrockToolConfig `json:"toolConfig"`
}

type bedrockToolConfig struct {
	Tools      []bedrockTool     `json:"tools"`
	ToolChoice bedrockToolChoice `json:"toolChoice"`
}

type bedrockTool struct {
	ToolSpec bedrockToolSpec `json:"toolSpec"`
}

type bedrockToolSpec struct {
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	InputSchema bedrockInputSchema `json:"inputSchema"`
}

type bedrockInputSchema struct {
	JSON map[string]any `json:"json"`
}

type bedrockToolChoice struct {
	Tool *bedrockToolChoiceSpec `json:"tool,omitempty"`
}

type bedrockToolChoiceSpec struct {
	Name string `json:"name"`
}

type bedrockToolUseResponse struct {
	Output struct {
		Message struct {
			Content []bedrockResponseContent `json:"content"`
		} `json:"message"`
	} `json:"output"`
}

type bedrockResponseContent struct {
	Text    *string              `json:"text,omitempty"`
	ToolUse *bedrockToolUseBlock `json:"toolUse,omitempty"`
}

type bedrockToolUseBlock struct {
	ToolUseID string         `json:"toolUseId"`
	Name      string         `json:"name"`
	Input     map[string]any `json:"input"`
}

func (c *BedrockLLMClient) post(ctx context.Context, model string, body []byte) ([]byte, error) {
	url := fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com/model/%s/converse", c.Region, model)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if err := signBedrockRequest(httpReq, body, c.Region); err != nil {
		return nil, fmt.Errorf("sign bedrock request: %w", err)
	}
	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("bedrock request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read bedrock response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bedrock returned %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	return raw, nil
}

func (c *BedrockLLMClient) generate(ctx context.Context, model, prompt string) (string, error) {
	req := bedrockConverseRequest{
		Messages: []bedrockMessage{
			{Role: "user", Content: []bedrockContentBlock{{Text: prompt}}},
		},
	}
	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal bedrock request: %w", err)
	}
	raw, err := c.post(ctx, model, body)
	if err != nil {
		return "", err
	}
	var result bedrockConverseResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("decode bedrock response: %w", err)
	}
	if len(result.Output.Message.Content) == 0 {
		return "", fmt.Errorf("bedrock returned empty content")
	}
	return strings.TrimSpace(result.Output.Message.Content[0].Text), nil
}

func (c *BedrockLLMClient) Generate(ctx context.Context, model, prompt string) (string, error) {
	return c.generate(ctx, model, prompt)
}

// GenerateJSON uses the Bedrock Converse toolConfig to enforce structured JSON output natively.
func (c *BedrockLLMClient) GenerateJSON(ctx context.Context, model, prompt string, schema map[string]any) (map[string]any, error) {
	req := bedrockToolUseRequest{
		Messages: []bedrockMessage{
			{Role: "user", Content: []bedrockContentBlock{{Text: prompt}}},
		},
		ToolConfig: bedrockToolConfig{
			Tools: []bedrockTool{{
				ToolSpec: bedrockToolSpec{
					Name:        "structured_output",
					Description: "Output structured data matching the provided schema.",
					InputSchema: bedrockInputSchema{JSON: schema},
				},
			}},
			ToolChoice: bedrockToolChoice{Tool: &bedrockToolChoiceSpec{Name: "structured_output"}},
		},
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal bedrock tool request: %w", err)
	}
	raw, err := c.post(ctx, model, body)
	if err != nil {
		return nil, err
	}
	var result bedrockToolUseResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decode bedrock tool response: %w", err)
	}
	for _, block := range result.Output.Message.Content {
		if block.ToolUse != nil && block.ToolUse.Name == "structured_output" {
			return block.ToolUse.Input, nil
		}
	}
	return nil, fmt.Errorf("bedrock did not return structured_output tool use block")
}

func (c *BedrockLLMClient) Evaluate(ctx context.Context, model, content, criteria string) (float64, error) {
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

// signBedrockRequest applies AWS SigV4 for the Bedrock service.
func signBedrockRequest(req *http.Request, body []byte, region string) error {
	return signAWSRequest(req, body, region, "bedrock")
}

// signAWSRequest applies AWS SigV4 to an HTTP request for the given service using env credentials.
func signAWSRequest(req *http.Request, body []byte, region, service string) error {
	accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	sessionToken := os.Getenv("AWS_SESSION_TOKEN")
	if accessKey == "" || secretKey == "" {
		return fmt.Errorf("AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY must be set")
	}

	now := time.Now().UTC()
	dateTime := now.Format("20060102T150405Z")
	date := now.Format("20060102")

	bodyHash := sha256Hex(body)
	req.Header.Set("X-Amz-Date", dateTime)
	req.Header.Set("X-Amz-Content-Sha256", bodyHash)
	if sessionToken != "" {
		req.Header.Set("X-Amz-Security-Token", sessionToken)
	}

	host := req.Host
	if host == "" {
		host = req.URL.Host
	}

	signedHeaders := "content-type;host;x-amz-content-sha256;x-amz-date"
	if sessionToken != "" {
		signedHeaders += ";x-amz-security-token"
	}

	canonicalHeaders := fmt.Sprintf(
		"content-type:%s\nhost:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n",
		req.Header.Get("Content-Type"), host, bodyHash, dateTime,
	)
	if sessionToken != "" {
		canonicalHeaders += fmt.Sprintf("x-amz-security-token:%s\n", sessionToken)
	}

	canonicalURI := req.URL.Path
	if canonicalURI == "" {
		canonicalURI = "/"
	}
	canonicalReq := strings.Join([]string{
		req.Method, canonicalURI, "",
		canonicalHeaders, signedHeaders, bodyHash,
	}, "\n")

	scope := strings.Join([]string{date, region, service, "aws4_request"}, "/")
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256", dateTime, scope, sha256Hex([]byte(canonicalReq)),
	}, "\n")

	signingKey := hmacSHA256(
		hmacSHA256(
			hmacSHA256(
				hmacSHA256([]byte("AWS4"+secretKey), []byte(date)),
				[]byte(region),
			),
			[]byte(service),
		),
		[]byte("aws4_request"),
	)
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	req.Header.Set("Authorization", fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		accessKey, scope, signedHeaders, signature,
	))
	return nil
}

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func hmacSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

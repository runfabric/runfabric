package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/runfabric/runfabric/platform/core/model/config"
)

// HTTPMCPClient calls MCP servers over JSON-RPC 2.0 HTTP transport.
type HTTPMCPClient struct {
	servers    map[string]string
	httpClient *http.Client
	nextID     atomic.Int64
}

// NewHTTPMCPClient builds an HTTPMCPClient from the MCP integrations config.
func NewHTTPMCPClient(integrations config.MCPIntegrationsConfig) *HTTPMCPClient {
	servers := make(map[string]string, len(integrations.Servers))
	for name, s := range integrations.Servers {
		servers[name] = strings.TrimRight(s.URL, "/")
	}
	return &HTTPMCPClient{
		servers:    servers,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

type jsonRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

type jsonRPCResponse struct {
	Result map[string]any `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (c *HTTPMCPClient) call(ctx context.Context, server, method string, params any) (map[string]any, error) {
	url, ok := c.servers[server]
	if !ok {
		return nil, fmt.Errorf("mcp server %q not configured", server)
	}
	reqBody, err := json.Marshal(jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      c.nextID.Add(1),
		Method:  method,
		Params:  params,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal mcp request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("mcp %s: %w", method, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read mcp response body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("mcp server returned %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	var result jsonRPCResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decode mcp response (status %d): %w", resp.StatusCode, err)
	}
	if result.Error != nil {
		return nil, fmt.Errorf("mcp %s error %d: %s", method, result.Error.Code, result.Error.Message)
	}
	if result.Result == nil {
		return map[string]any{}, nil
	}
	return result.Result, nil
}

func (c *HTTPMCPClient) CallTool(ctx context.Context, server, name string, args map[string]any) (map[string]any, error) {
	return c.call(ctx, server, "tools/call", map[string]any{
		"name":      name,
		"arguments": args,
	})
}

func (c *HTTPMCPClient) ReadResource(ctx context.Context, server, uri string) (map[string]any, error) {
	return c.call(ctx, server, "resources/read", map[string]any{
		"uri": uri,
	})
}

func (c *HTTPMCPClient) GetPrompt(ctx context.Context, server, ref string, args map[string]any) (map[string]any, error) {
	return c.call(ctx, server, "prompts/get", map[string]any{
		"name":      ref,
		"arguments": args,
	})
}

// NoopMCPClient is a deterministic MCP client stub for local runtime/testing.
type NoopMCPClient struct{}

func (NoopMCPClient) CallTool(_ context.Context, server, name string, args map[string]any) (map[string]any, error) {
	return map[string]any{
		"type":   "tool",
		"server": server,
		"name":   name,
		"args":   args,
	}, nil
}

func (NoopMCPClient) ReadResource(_ context.Context, server, uri string) (map[string]any, error) {
	return map[string]any{
		"type":   "resource",
		"server": server,
		"uri":    uri,
		"value":  fmt.Sprintf("resource:%s", uri),
	}, nil
}

func (NoopMCPClient) GetPrompt(_ context.Context, server, ref string, args map[string]any) (map[string]any, error) {
	return map[string]any{
		"type":   "prompt",
		"server": server,
		"ref":    ref,
		"text":   fmt.Sprintf("prompt:%s", ref),
		"args":   args,
	}, nil
}

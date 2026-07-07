package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

func (p *plugin) Invoke(ctx context.Context, req sdkprovider.InvokeRequest) (*sdkprovider.InvokeResult, error) {
	if _, _, _, err := p.inspectConfig(req.Config); err != nil {
		return nil, err
	}
	functionName := p.resolveFunctionName(req.Config, req.Function)
	if invokeURL := p.resolveInvokeURL(req.Config, functionName); invokeURL != "" {
		return p.invokeHTTP(ctx, invokeURL, functionName, req.Payload)
	}
	out, err := p.executeOperation(ctx, "", req.Config, "invoke", req.Stage, functionName, req.Payload)
	if err != nil {
		return nil, err
	}
	if parsed, ok := parseInvokeResult(out); ok {
		if parsed.Provider == "" {
			parsed.Provider = p.provider
		}
		if parsed.Function == "" {
			parsed.Function = functionName
		}
		return parsed, nil
	}
	return &sdkprovider.InvokeResult{
		Provider: p.provider,
		Function: functionName,
		Output:   strings.TrimSpace(string(out)),
	}, nil
}

func (p *plugin) invokeHTTP(ctx context.Context, url, function string, payload []byte) (*sdkprovider.InvokeResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	contentType := "application/octet-stream"
	if json.Valid(payload) || len(payload) == 0 {
		contentType = "application/json"
	}
	req.Header.Set("Content-Type", contentType)
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("invoke function over HTTP: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	output := strings.TrimSpace(string(body))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &sdkprovider.InvokeResult{Provider: p.provider, Function: function, Output: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, output)}, nil
	}
	return &sdkprovider.InvokeResult{
		Provider: p.provider,
		Function: function,
		Output:   output,
		RunID:    strings.TrimSpace(resp.Header.Get("X-Request-Id")),
	}, nil
}

func parseInvokeResult(out []byte) (*sdkprovider.InvokeResult, bool) {
	var result sdkprovider.InvokeResult
	if json.Unmarshal(out, &result) != nil {
		return nil, false
	}
	if result.Output == "" && result.RunID == "" && result.Workflow == "" {
		return nil, false
	}
	return &result, true
}

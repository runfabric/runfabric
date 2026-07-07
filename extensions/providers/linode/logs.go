package main

import (
	"context"
	"encoding/json"
	"strings"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

func (p *plugin) Logs(ctx context.Context, req sdkprovider.LogsRequest) (*sdkprovider.LogsResult, error) {
	if _, _, _, err := p.inspectConfig(req.Config); err != nil {
		return nil, err
	}
	functionName := p.resolveFunctionName(req.Config, req.Function)
	out, err := p.executeOperation(ctx, "", req.Config, "logs", req.Stage, functionName, nil)
	if err != nil {
		return nil, err
	}
	if parsed, ok := parseLogsResult(out); ok {
		if parsed.Provider == "" {
			parsed.Provider = p.provider
		}
		if parsed.Function == "" {
			parsed.Function = functionName
		}
		return parsed, nil
	}
	lines := splitOutputLines(out)
	if len(lines) == 0 {
		lines = []string{"no log output"}
	}
	return &sdkprovider.LogsResult{Provider: p.provider, Function: functionName, Lines: lines}, nil
}

func parseLogsResult(out []byte) (*sdkprovider.LogsResult, bool) {
	var result sdkprovider.LogsResult
	if json.Unmarshal(out, &result) == nil && len(result.Lines) > 0 {
		return &result, true
	}
	var lines []string
	if json.Unmarshal(out, &lines) == nil && len(lines) > 0 {
		return &sdkprovider.LogsResult{Lines: lines}, true
	}
	return nil, false
}

func splitOutputLines(out []byte) []string {
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return nil
	}
	rawLines := strings.Split(trimmed, "\n")
	lines := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

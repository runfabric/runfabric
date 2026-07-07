package main

import (
	"context"
	"encoding/json"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

func (p *plugin) Remove(ctx context.Context, req sdkprovider.RemoveRequest) (*sdkprovider.RemoveResult, error) {
	if _, _, _, err := p.inspectConfig(req.Config); err != nil {
		return nil, err
	}
	out, err := p.executeOperation(ctx, req.Root, req.Config, "remove", req.Stage, "", nil)
	if err != nil {
		return nil, err
	}
	if parsed, ok := parseRemoveResult(out); ok {
		if parsed.Provider == "" {
			parsed.Provider = p.provider
		}
		return parsed, nil
	}
	return &sdkprovider.RemoveResult{Provider: p.provider, Removed: true}, nil
}

func parseRemoveResult(out []byte) (*sdkprovider.RemoveResult, bool) {
	var result sdkprovider.RemoveResult
	if json.Unmarshal(out, &result) != nil {
		return nil, false
	}
	if result.Provider == "" && !result.Removed {
		return nil, false
	}
	return &result, true
}

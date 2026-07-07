package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

func (p *plugin) Deploy(ctx context.Context, req sdkprovider.DeployRequest) (*sdkprovider.DeployResult, error) {
	service, functions, _, err := p.inspectConfig(req.Config)
	if err != nil {
		return nil, err
	}
	out, err := p.executeOperation(ctx, req.Root, req.Config, "deploy", req.Stage, "", nil)
	if err != nil {
		return nil, err
	}
	if parsed, ok := parseDeployResult(out); ok {
		if parsed.Provider == "" {
			parsed.Provider = p.provider
		}
		if parsed.DeploymentID == "" {
			parsed.DeploymentID = p.defaultDeploymentID(service, req.Stage)
		}
		return parsed, nil
	}
	result := &sdkprovider.DeployResult{
		Provider:     p.provider,
		DeploymentID: p.defaultDeploymentID(service, req.Stage),
		Outputs: map[string]string{
			"stdout": strings.TrimSpace(string(out)),
		},
		Metadata: map[string]string{
			"mode":    "command",
			"service": service,
			"stage":   req.Stage,
		},
		Functions: make(map[string]sdkprovider.DeployedFunction, len(functions)),
	}
	for _, fn := range functions {
		result.Functions[fn.Name] = sdkprovider.DeployedFunction{ResourceName: fn.Name}
	}
	return result, nil
}

func (p *plugin) defaultDeploymentID(service, stage string) string {
	return fmt.Sprintf("linode-%s-%s-%d", service, stage, p.deploymentNow().Unix())
}

func parseDeployResult(out []byte) (*sdkprovider.DeployResult, bool) {
	var result sdkprovider.DeployResult
	if json.Unmarshal(out, &result) != nil {
		return nil, false
	}
	if result.DeploymentID == "" && len(result.Outputs) == 0 && len(result.Metadata) == 0 && len(result.Functions) == 0 && len(result.Artifacts) == 0 {
		return nil, false
	}
	return &result, true
}

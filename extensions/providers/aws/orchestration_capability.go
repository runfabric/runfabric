package aws

import (
	"context"
	"fmt"
	"strings"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

func (p *Provider) SyncOrchestrations(ctx context.Context, req sdkprovider.OrchestrationSyncRequest) (*sdkprovider.OrchestrationSyncResult, error) {
	_ = ctx
	_ = req
	return &sdkprovider.OrchestrationSyncResult{
		Metadata: map[string]string{"provider": ProviderID, "status": "synced"},
		Outputs:  map[string]string{},
	}, nil
}

func (p *Provider) RemoveOrchestrations(ctx context.Context, req sdkprovider.OrchestrationRemoveRequest) (*sdkprovider.OrchestrationSyncResult, error) {
	_ = ctx
	_ = req
	return &sdkprovider.OrchestrationSyncResult{
		Metadata: map[string]string{"provider": ProviderID, "status": "removed"},
		Outputs:  map[string]string{},
	}, nil
}

func (p *Provider) InvokeOrchestration(ctx context.Context, req sdkprovider.OrchestrationInvokeRequest) (*sdkprovider.InvokeResult, error) {
	_ = ctx
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("orchestration name is required")
	}
	return &sdkprovider.InvokeResult{
		Provider: p.Name(),
		Function: "sfn:" + name,
		Output:   "orchestration invocation accepted",
		Workflow: name,
	}, nil
}

func (p *Provider) InspectOrchestrations(ctx context.Context, req sdkprovider.OrchestrationInspectRequest) (map[string]any, error) {
	_ = ctx
	_ = req
	return map[string]any{"stepFunctions": []any{}}, nil
}

func executionConsoleLink(region, executionArn string) string {
	if strings.TrimSpace(region) == "" || strings.TrimSpace(executionArn) == "" {
		return ""
	}
	return "https://" + region + ".console.aws.amazon.com/states/home?region=" + region + "#/executions/details/" + executionArn
}

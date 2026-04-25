package aws

import (
	"context"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

func DeployAPIOps(ctx context.Context, cfg sdkprovider.Config, stage, root string) (*sdkprovider.DeployResult, error) {
	return (&Provider{}).Deploy(ctx, sdkprovider.DeployRequest{Config: cfg, Stage: stage, Root: root})
}

func RemoveAPIOps(ctx context.Context, cfg sdkprovider.Config, stage, root string, receipt any) (*sdkprovider.RemoveResult, error) {
	return (&Provider{}).Remove(ctx, sdkprovider.RemoveRequest{Config: cfg, Stage: stage, Root: root, Receipt: receipt})
}

func InvokeAPIOps(ctx context.Context, cfg sdkprovider.Config, stage, function string, payload []byte, receipt any) (*sdkprovider.InvokeResult, error) {
	_ = receipt
	return (&Provider{}).Invoke(ctx, sdkprovider.InvokeRequest{Config: cfg, Stage: stage, Function: function, Payload: payload})
}

func LogsAPIOps(ctx context.Context, cfg sdkprovider.Config, stage, function string, receipt any) (*sdkprovider.LogsResult, error) {
	_ = receipt
	return (&Provider{}).Logs(ctx, sdkprovider.LogsRequest{Config: cfg, Stage: stage, Function: function})
}

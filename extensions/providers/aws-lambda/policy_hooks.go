package aws

import (
	"context"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

func PrepareDevStreamPolicy(ctx context.Context, cfg sdkprovider.Config, stage, tunnelURL string) (*sdkprovider.DevStreamSession, error) {
	return (&Provider{}).PrepareDevStream(ctx, sdkprovider.DevStreamRequest{Config: cfg, Stage: stage, TunnelURL: tunnelURL})
}

func FetchMetricsPolicy(ctx context.Context, cfg sdkprovider.Config, stage string) (*sdkprovider.MetricsResult, error) {
	return (&Provider{}).FetchMetrics(ctx, sdkprovider.MetricsRequest{Config: cfg, Stage: stage})
}

func FetchTracesPolicy(ctx context.Context, cfg sdkprovider.Config, stage string) (*sdkprovider.TracesResult, error) {
	return (&Provider{}).FetchTraces(ctx, sdkprovider.TracesRequest{Config: cfg, Stage: stage})
}

func RecoverPolicy(ctx context.Context, req sdkprovider.RecoveryRequest) (*sdkprovider.RecoveryResult, error) {
	return (&Provider{}).Recover(ctx, req)
}

func SyncOrchestrationsPolicy(ctx context.Context, req sdkprovider.OrchestrationSyncRequest) (*sdkprovider.OrchestrationSyncResult, error) {
	return (&Provider{}).SyncOrchestrations(ctx, req)
}

func RemoveOrchestrationsPolicy(ctx context.Context, req sdkprovider.OrchestrationRemoveRequest) (*sdkprovider.OrchestrationSyncResult, error) {
	return (&Provider{}).RemoveOrchestrations(ctx, req)
}

func InvokeOrchestrationPolicy(ctx context.Context, req sdkprovider.OrchestrationInvokeRequest) (*sdkprovider.InvokeResult, error) {
	return (&Provider{}).InvokeOrchestration(ctx, req)
}

func InspectOrchestrationsPolicy(ctx context.Context, req sdkprovider.OrchestrationInspectRequest) (map[string]any, error) {
	return (&Provider{}).InspectOrchestrations(ctx, req)
}

package inprocess

import (
	"context"

	providers "github.com/runfabric/runfabric/platform/core/contracts/provider"
)

// APIDispatchHooks provides optional provider-specific capabilities that are not
// part of deploy/remove/invoke/logs primitives.
type APIDispatchHooks struct {
	PrepareDevStream      func(ctx context.Context, cfg providers.Config, stage, tunnelURL string) (*providers.DevStreamSession, error)
	FetchMetrics          func(ctx context.Context, cfg providers.Config, stage string) (*providers.MetricsResult, error)
	FetchTraces           func(ctx context.Context, cfg providers.Config, stage string) (*providers.TracesResult, error)
	Recover               func(ctx context.Context, req providers.RecoveryRequest) (*providers.RecoveryResult, error)
	SyncOrchestrations    func(ctx context.Context, req providers.OrchestrationSyncRequest) (*providers.OrchestrationSyncResult, error)
	RemoveOrchestrations  func(ctx context.Context, req providers.OrchestrationRemoveRequest) (*providers.OrchestrationSyncResult, error)
	InvokeOrchestration   func(ctx context.Context, req providers.OrchestrationInvokeRequest) (*providers.InvokeResult, error)
	InspectOrchestrations func(ctx context.Context, req providers.OrchestrationInspectRequest) (map[string]any, error)
}

// APIOps contains required API-dispatch operations.
type APIOps struct {
	Deploy func(ctx context.Context, cfg providers.Config, stage, root string) (*providers.DeployResult, error)
	Remove func(ctx context.Context, cfg providers.Config, stage, root string, receipt any) (*providers.RemoveResult, error)
	Invoke func(ctx context.Context, cfg providers.Config, stage, function string, payload []byte, receipt any) (*providers.InvokeResult, error)
	Logs   func(ctx context.Context, cfg providers.Config, stage, function string, receipt any) (*providers.LogsResult, error)
}

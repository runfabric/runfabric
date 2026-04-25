package azure

import (
	"context"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// TransportPlugin implements sdkprovider.Plugin for Azure Functions.
type TransportPlugin struct{}

func NewTransportPlugin() *TransportPlugin { return &TransportPlugin{} }

func (tp *TransportPlugin) Meta() sdkprovider.Meta {
	return sdkprovider.Meta{
		Name:            "azure-functions",
		PluginVersion:   "1",
		Capabilities:    []string{"remove", "invoke", "logs", "doctor", "plan"},
		SupportsRuntime: []string{"nodejs", "python"},
	}
}

func (tp *TransportPlugin) ValidateConfig(ctx context.Context, req sdkprovider.ValidateConfigRequest) error {
	_ = ctx
	_ = req
	return nil
}

func (tp *TransportPlugin) Doctor(ctx context.Context, req sdkprovider.DoctorRequest) (*sdkprovider.DoctorResult, error) {
	return &sdkprovider.DoctorResult{Provider: "azure-functions", Checks: []string{"azure plugin loaded"}}, nil
}

func (tp *TransportPlugin) Plan(ctx context.Context, req sdkprovider.PlanRequest) (*sdkprovider.PlanResult, error) {
	return &sdkprovider.PlanResult{Provider: "azure-functions", Plan: map[string]any{"provider": "azure-functions", "stage": req.Stage, "root": req.Root}}, nil
}

func (tp *TransportPlugin) Deploy(ctx context.Context, req sdkprovider.DeployRequest) (*sdkprovider.DeployResult, error) {
	return (Runner{}).Deploy(ctx, req.Config, req.Stage, req.Root)
}

func (tp *TransportPlugin) Remove(ctx context.Context, req sdkprovider.RemoveRequest) (*sdkprovider.RemoveResult, error) {
	return (Remover{}).Remove(ctx, req.Config, req.Stage, req.Root, req.Receipt)
}

func (tp *TransportPlugin) Invoke(ctx context.Context, req sdkprovider.InvokeRequest) (*sdkprovider.InvokeResult, error) {
	return (Invoker{}).Invoke(ctx, req.Config, req.Stage, req.Function, req.Payload, nil)
}

func (tp *TransportPlugin) Logs(ctx context.Context, req sdkprovider.LogsRequest) (*sdkprovider.LogsResult, error) {
	return (Logger{}).Logs(ctx, req.Config, req.Stage, req.Function, nil)
}

func (tp *TransportPlugin) PrepareDevStream(ctx context.Context, req sdkprovider.DevStreamRequest) (*sdkprovider.DevStreamSession, error) {
	return PrepareDevStreamPolicy(ctx, req.Config, req.Stage, req.TunnelURL)
}

func (tp *TransportPlugin) FetchMetrics(ctx context.Context, req sdkprovider.MetricsRequest) (*sdkprovider.MetricsResult, error) {
	return FetchMetricsPolicy(ctx, req.Config, req.Stage)
}

func (tp *TransportPlugin) FetchTraces(ctx context.Context, req sdkprovider.TracesRequest) (*sdkprovider.TracesResult, error) {
	return FetchTracesPolicy(ctx, req.Config, req.Stage)
}

func (tp *TransportPlugin) Recover(ctx context.Context, req sdkprovider.RecoveryRequest) (*sdkprovider.RecoveryResult, error) {
	return RecoverPolicy(ctx, req)
}

func (tp *TransportPlugin) SyncOrchestrations(ctx context.Context, req sdkprovider.OrchestrationSyncRequest) (*sdkprovider.OrchestrationSyncResult, error) {
	return SyncOrchestrationsPolicy(ctx, req)
}

func (tp *TransportPlugin) RemoveOrchestrations(ctx context.Context, req sdkprovider.OrchestrationRemoveRequest) (*sdkprovider.OrchestrationSyncResult, error) {
	return RemoveOrchestrationsPolicy(ctx, req)
}

func (tp *TransportPlugin) InvokeOrchestration(ctx context.Context, req sdkprovider.OrchestrationInvokeRequest) (*sdkprovider.InvokeResult, error) {
	return InvokeOrchestrationPolicy(ctx, req)
}

func (tp *TransportPlugin) InspectOrchestrations(ctx context.Context, req sdkprovider.OrchestrationInspectRequest) (map[string]any, error) {
	return InspectOrchestrationsPolicy(ctx, req)
}

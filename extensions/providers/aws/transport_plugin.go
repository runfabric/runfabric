// transport_plugin.go implements sdkprovider.Plugin for AWS Lambda.
// It delegates all operations directly to Provider, which now uses SDK types natively.
package aws

import (
	"context"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// TransportPlugin implements sdkprovider.Plugin for AWS Lambda.
// Wrap it with inprocess.New to register it as a core ProviderPlugin.
type TransportPlugin struct{}

// NewTransportPlugin returns a TransportPlugin ready to be wrapped by inprocess.New.
func NewTransportPlugin() *TransportPlugin {
	return &TransportPlugin{}
}

func (tp *TransportPlugin) Meta() sdkprovider.Meta {
	return sdkprovider.Meta{
		Name:            ProviderID,
		PluginVersion:   "1",
		Capabilities:    []string{"remove", "invoke", "logs", "doctor", "plan"},
		SupportsRuntime: []string{"nodejs", "python"},
	}
}

func (tp *TransportPlugin) ValidateConfig(ctx context.Context, req sdkprovider.ValidateConfigRequest) error {
	return (&Provider{}).ValidateConfig(ctx, req)
}

func (tp *TransportPlugin) Doctor(ctx context.Context, req sdkprovider.DoctorRequest) (*sdkprovider.DoctorResult, error) {
	return (&Provider{}).Doctor(ctx, req)
}

func (tp *TransportPlugin) Plan(ctx context.Context, req sdkprovider.PlanRequest) (*sdkprovider.PlanResult, error) {
	return (&Provider{}).Plan(ctx, req)
}

func (tp *TransportPlugin) Deploy(ctx context.Context, req sdkprovider.DeployRequest) (*sdkprovider.DeployResult, error) {
	return (&Provider{}).Deploy(ctx, req)
}

func (tp *TransportPlugin) Remove(ctx context.Context, req sdkprovider.RemoveRequest) (*sdkprovider.RemoveResult, error) {
	return (&Provider{}).Remove(ctx, req)
}

func (tp *TransportPlugin) Invoke(ctx context.Context, req sdkprovider.InvokeRequest) (*sdkprovider.InvokeResult, error) {
	return (&Provider{}).Invoke(ctx, req)
}

func (tp *TransportPlugin) Logs(ctx context.Context, req sdkprovider.LogsRequest) (*sdkprovider.LogsResult, error) {
	return (&Provider{}).Logs(ctx, req)
}

// FetchMetrics implements sdkprovider.ObservabilityCapable.
// Provider.FetchMetrics now uses SDK types natively — no conversion needed.
func (tp *TransportPlugin) FetchMetrics(ctx context.Context, req sdkprovider.MetricsRequest) (*sdkprovider.MetricsResult, error) {
	return (&Provider{}).FetchMetrics(ctx, req)
}

// FetchTraces implements sdkprovider.ObservabilityCapable.
// Provider.FetchTraces now uses SDK types natively — no conversion needed.
func (tp *TransportPlugin) FetchTraces(ctx context.Context, req sdkprovider.TracesRequest) (*sdkprovider.TracesResult, error) {
	return (&Provider{}).FetchTraces(ctx, req)
}

// PrepareDevStreamLocal satisfies the inprocess.localDevStreamCapable interface.
// Provider.PrepareDevStream now uses SDK types natively and returns *sdkprovider.DevStreamSession
// which carries the in-memory restore callback via sdkprovider.NewDevStreamSession.
func (tp *TransportPlugin) PrepareDevStreamLocal(ctx context.Context, req sdkprovider.DevStreamRequest) (*sdkprovider.DevStreamSession, error) {
	return (&Provider{}).PrepareDevStream(ctx, req)
}

// Recover implements sdkprovider.RecoveryCapable.
// Provider.Recover now uses SDK types natively — no conversion needed.
func (tp *TransportPlugin) Recover(ctx context.Context, req sdkprovider.RecoveryRequest) (*sdkprovider.RecoveryResult, error) {
	return (&Provider{}).Recover(ctx, req)
}

func (tp *TransportPlugin) SyncOrchestrations(ctx context.Context, req sdkprovider.OrchestrationSyncRequest) (*sdkprovider.OrchestrationSyncResult, error) {
	return (&Provider{}).SyncOrchestrations(ctx, req)
}

func (tp *TransportPlugin) RemoveOrchestrations(ctx context.Context, req sdkprovider.OrchestrationRemoveRequest) (*sdkprovider.OrchestrationSyncResult, error) {
	return (&Provider{}).RemoveOrchestrations(ctx, req)
}

func (tp *TransportPlugin) InvokeOrchestration(ctx context.Context, req sdkprovider.OrchestrationInvokeRequest) (*sdkprovider.InvokeResult, error) {
	return (&Provider{}).InvokeOrchestration(ctx, req)
}

func (tp *TransportPlugin) InspectOrchestrations(ctx context.Context, req sdkprovider.OrchestrationInspectRequest) (map[string]any, error) {
	return (&Provider{}).InspectOrchestrations(ctx, req)
}

// transport_plugin.go implements sdkprovider.Plugin for AWS Lambda.
// It delegates all operations to the existing Provider methods after config conversion,
// exposing the optional transport capabilities supported by this provider.
//
// This file is the migration shim that allows the AWS provider to be used via
// the inprocess.Adapter without modifying the core Provider implementation.
package aws

import (
	"context"

	coreprovider "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	"github.com/runfabric/runfabric/platform/extensions/sdkbridge"
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
		Capabilities:    []string{"deploy", "remove", "invoke", "logs", "doctor", "plan"},
		SupportsRuntime: []string{"nodejs", "python"},
	}
}

func (tp *TransportPlugin) ValidateConfig(ctx context.Context, req sdkprovider.ValidateConfigRequest) error {
	cfg, err := sdkbridge.ToCoreConfig(req.Config)
	if err != nil {
		return err
	}
	return (&Provider{}).ValidateConfig(ctx, coreprovider.ValidateConfigRequest{Config: cfg, Stage: req.Stage})
}

func (tp *TransportPlugin) Doctor(ctx context.Context, req sdkprovider.DoctorRequest) (*sdkprovider.DoctorResult, error) {
	cfg, err := sdkbridge.ToCoreConfig(req.Config)
	if err != nil {
		return nil, err
	}
	r, err := (&Provider{}).Doctor(ctx, coreprovider.DoctorRequest{Config: cfg, Stage: req.Stage})
	if err != nil {
		return nil, err
	}
	return &sdkprovider.DoctorResult{Provider: r.Provider, Checks: r.Checks}, nil
}

func (tp *TransportPlugin) Plan(ctx context.Context, req sdkprovider.PlanRequest) (*sdkprovider.PlanResult, error) {
	cfg, err := sdkbridge.ToCoreConfig(req.Config)
	if err != nil {
		return nil, err
	}
	r, err := (&Provider{}).Plan(ctx, coreprovider.PlanRequest{Config: cfg, Stage: req.Stage, Root: req.Root})
	if err != nil {
		return nil, err
	}
	return &sdkprovider.PlanResult{Provider: r.Provider, Plan: r.Plan, Warnings: r.Warnings}, nil
}

func (tp *TransportPlugin) Deploy(ctx context.Context, req sdkprovider.DeployRequest) (*sdkprovider.DeployResult, error) {
	cfg, err := sdkbridge.ToCoreConfig(req.Config)
	if err != nil {
		return nil, err
	}
	r, err := (&Provider{}).Deploy(ctx, coreprovider.DeployRequest{Config: cfg, Stage: req.Stage, Root: req.Root})
	if err != nil {
		return nil, err
	}
	out := &sdkprovider.DeployResult{
		Provider:     r.Provider,
		DeploymentID: r.DeploymentID,
		Outputs:      r.Outputs,
		Metadata:     r.Metadata,
	}
	if len(r.Artifacts) > 0 {
		out.Artifacts = make([]sdkprovider.Artifact, len(r.Artifacts))
		for i, art := range r.Artifacts {
			out.Artifacts[i] = sdkprovider.Artifact{
				Function:        art.Function,
				Runtime:         art.Runtime,
				SourcePath:      art.SourcePath,
				OutputPath:      art.OutputPath,
				SHA256:          art.SHA256,
				SizeBytes:       art.SizeBytes,
				ConfigSignature: art.ConfigSignature,
			}
		}
	}
	if len(r.Functions) > 0 {
		out.Functions = make(map[string]sdkprovider.DeployedFunction, len(r.Functions))
		for k, f := range r.Functions {
			out.Functions[k] = sdkprovider.DeployedFunction{
				ResourceName:       f.ResourceName,
				ResourceIdentifier: f.ResourceIdentifier,
				Metadata:           f.Metadata,
			}
		}
	}
	return out, nil
}

func (tp *TransportPlugin) Remove(ctx context.Context, req sdkprovider.RemoveRequest) (*sdkprovider.RemoveResult, error) {
	cfg, err := sdkbridge.ToCoreConfig(req.Config)
	if err != nil {
		return nil, err
	}
	r, err := (&Provider{}).Remove(ctx, coreprovider.RemoveRequest{
		Config: cfg, Stage: req.Stage, Root: req.Root, Receipt: req.Receipt,
	})
	if err != nil {
		return nil, err
	}
	return &sdkprovider.RemoveResult{Provider: r.Provider, Removed: r.Removed}, nil
}

func (tp *TransportPlugin) Invoke(ctx context.Context, req sdkprovider.InvokeRequest) (*sdkprovider.InvokeResult, error) {
	cfg, err := sdkbridge.ToCoreConfig(req.Config)
	if err != nil {
		return nil, err
	}
	r, err := (&Provider{}).Invoke(ctx, coreprovider.InvokeRequest{
		Config: cfg, Stage: req.Stage, Function: req.Function, Payload: req.Payload,
	})
	if err != nil {
		return nil, err
	}
	return &sdkprovider.InvokeResult{
		Provider: r.Provider, Function: r.Function, Output: r.Output,
		RunID: r.RunID, Workflow: r.Workflow,
	}, nil
}

func (tp *TransportPlugin) Logs(ctx context.Context, req sdkprovider.LogsRequest) (*sdkprovider.LogsResult, error) {
	cfg, err := sdkbridge.ToCoreConfig(req.Config)
	if err != nil {
		return nil, err
	}
	r, err := (&Provider{}).Logs(ctx, coreprovider.LogsRequest{
		Config: cfg, Stage: req.Stage, Function: req.Function,
	})
	if err != nil {
		return nil, err
	}
	return &sdkprovider.LogsResult{
		Provider: r.Provider, Function: r.Function, Lines: r.Lines, Workflow: r.Workflow,
	}, nil
}

// FetchMetrics implements sdkprovider.ObservabilityCapable.
func (tp *TransportPlugin) FetchMetrics(ctx context.Context, req sdkprovider.MetricsRequest) (*sdkprovider.MetricsResult, error) {
	cfg, err := sdkbridge.ToCoreConfig(req.Config)
	if err != nil {
		return nil, err
	}
	r, err := (&Provider{}).FetchMetrics(ctx, coreprovider.MetricsRequest{Config: cfg, Stage: req.Stage})
	if err != nil {
		return nil, err
	}
	return &sdkprovider.MetricsResult{PerFunction: r.PerFunction, Message: r.Message}, nil
}

// FetchTraces implements sdkprovider.ObservabilityCapable.
func (tp *TransportPlugin) FetchTraces(ctx context.Context, req sdkprovider.TracesRequest) (*sdkprovider.TracesResult, error) {
	cfg, err := sdkbridge.ToCoreConfig(req.Config)
	if err != nil {
		return nil, err
	}
	r, err := (&Provider{}).FetchTraces(ctx, coreprovider.TracesRequest{Config: cfg, Stage: req.Stage})
	if err != nil {
		return nil, err
	}
	return &sdkprovider.TracesResult{Traces: r.Traces, Message: r.Message}, nil
}

// PrepareDevStreamLocal satisfies the inprocess.localDevStreamCapable interface,
// preserving the in-memory restore callback so it is not lost at the transport boundary.
func (tp *TransportPlugin) PrepareDevStreamLocal(ctx context.Context, req sdkprovider.DevStreamRequest) (*coreprovider.DevStreamSession, error) {
	cfg, err := sdkbridge.ToCoreConfig(req.Config)
	if err != nil {
		return nil, err
	}
	return (&Provider{}).PrepareDevStream(ctx, coreprovider.DevStreamRequest{
		Config:    cfg,
		Stage:     req.Stage,
		TunnelURL: req.TunnelURL,
		Region:    req.Region,
	})
}

// Recover implements sdkprovider.RecoveryCapable.
func (tp *TransportPlugin) Recover(ctx context.Context, req sdkprovider.RecoveryRequest) (*sdkprovider.RecoveryResult, error) {
	cfg, err := sdkbridge.ToCoreConfig(req.Config)
	if err != nil {
		return nil, err
	}
	r, err := (&Provider{}).Recover(ctx, coreprovider.RecoveryRequest{
		Config: cfg, Root: req.Root, Service: req.Service,
		Stage: req.Stage, Region: req.Region, Mode: req.Mode, Journal: req.Journal,
	})
	if err != nil {
		return nil, err
	}
	return &sdkprovider.RecoveryResult{
		Recovered: r.Recovered, Mode: r.Mode, Status: r.Status, Message: r.Message,
		Metadata: r.Metadata, Errors: r.Errors, ResumeData: r.ResumeData,
	}, nil
}

// SyncOrchestrations implements sdkprovider.OrchestrationCapable.
func (tp *TransportPlugin) SyncOrchestrations(ctx context.Context, req sdkprovider.OrchestrationSyncRequest) (*sdkprovider.OrchestrationSyncResult, error) {
	cfg, err := sdkbridge.ToCoreConfig(req.Config)
	if err != nil {
		return nil, err
	}
	r, err := (&Provider{}).SyncOrchestrations(ctx, coreprovider.OrchestrationSyncRequest{
		Config: cfg, Stage: req.Stage, Root: req.Root,
		FunctionResourceByName: req.FunctionResourceByName,
	})
	if err != nil {
		return nil, err
	}
	return &sdkprovider.OrchestrationSyncResult{Metadata: r.Metadata, Outputs: r.Outputs}, nil
}

// RemoveOrchestrations implements sdkprovider.OrchestrationCapable.
func (tp *TransportPlugin) RemoveOrchestrations(ctx context.Context, req sdkprovider.OrchestrationRemoveRequest) (*sdkprovider.OrchestrationSyncResult, error) {
	cfg, err := sdkbridge.ToCoreConfig(req.Config)
	if err != nil {
		return nil, err
	}
	r, err := (&Provider{}).RemoveOrchestrations(ctx, coreprovider.OrchestrationRemoveRequest{
		Config: cfg, Stage: req.Stage, Root: req.Root,
	})
	if err != nil {
		return nil, err
	}
	return &sdkprovider.OrchestrationSyncResult{Metadata: r.Metadata, Outputs: r.Outputs}, nil
}

// InvokeOrchestration implements sdkprovider.OrchestrationCapable.
func (tp *TransportPlugin) InvokeOrchestration(ctx context.Context, req sdkprovider.OrchestrationInvokeRequest) (*sdkprovider.InvokeResult, error) {
	cfg, err := sdkbridge.ToCoreConfig(req.Config)
	if err != nil {
		return nil, err
	}
	r, err := (&Provider{}).InvokeOrchestration(ctx, coreprovider.OrchestrationInvokeRequest{
		Config: cfg, Stage: req.Stage, Root: req.Root, Name: req.Name, Payload: req.Payload,
	})
	if err != nil {
		return nil, err
	}
	return &sdkprovider.InvokeResult{
		Provider: r.Provider, Function: r.Function, Output: r.Output,
		RunID: r.RunID, Workflow: r.Workflow,
	}, nil
}

// InspectOrchestrations implements sdkprovider.OrchestrationCapable.
func (tp *TransportPlugin) InspectOrchestrations(ctx context.Context, req sdkprovider.OrchestrationInspectRequest) (map[string]any, error) {
	cfg, err := sdkbridge.ToCoreConfig(req.Config)
	if err != nil {
		return nil, err
	}
	return (&Provider{}).InspectOrchestrations(ctx, coreprovider.OrchestrationInspectRequest{
		Config: cfg, Stage: req.Stage, Root: req.Root,
	})
}

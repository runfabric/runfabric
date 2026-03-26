// Package inprocess provides an Adapter that wraps a sdkprovider.Plugin to satisfy the
// engine's core providers.ProviderPlugin contract, allowing built-in provider
// implementations to adopt the transport-safe plugin interface without coupling
// themselves to engine internals.
package inprocess

import (
	"context"
	"encoding/json"

	sdkbridge "github.com/runfabric/runfabric/internal/provider/sdkbridge"
	coreprovider "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	planner "github.com/runfabric/runfabric/platform/core/planner/api"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// localDevStreamCapable is satisfied by in-process sdkprovider.Plugin implementations
// that hold a live restore callback and can return an SDK DevStreamSession directly.
// The Adapter checks for this interface before falling back to the serialisable
// sdkprovider.DevStreamCapable path. The SDK DevStreamSession may carry an unexported
// restore func set via sdkprovider.NewDevStreamSession for in-process use.
type localDevStreamCapable interface {
	PrepareDevStreamLocal(ctx context.Context, req sdkprovider.DevStreamRequest) (*sdkprovider.DevStreamSession, error)
}

// Adapter wraps a sdkprovider.Plugin to satisfy the engine's core ProviderPlugin contract.
type Adapter struct {
	plugin sdkprovider.Plugin
}

// New returns an Adapter backed by the given sdkprovider.Plugin.
func New(plugin sdkprovider.Plugin) *Adapter {
	return &Adapter{plugin: plugin}
}

func (a *Adapter) Meta() coreprovider.ProviderMeta {
	m := a.plugin.Meta()
	return coreprovider.ProviderMeta{
		Name:              m.Name,
		Version:           m.Version,
		PluginVersion:     m.PluginVersion,
		Capabilities:      append([]string(nil), m.Capabilities...),
		SupportsRuntime:   append([]string(nil), m.SupportsRuntime...),
		SupportsTriggers:  append([]string(nil), m.SupportsTriggers...),
		SupportsResources: append([]string(nil), m.SupportsResources...),
	}
}

func (a *Adapter) ValidateConfig(ctx context.Context, req coreprovider.ValidateConfigRequest) error {
	tc, err := sdkbridge.FromCoreConfig(req.Config)
	if err != nil {
		return err
	}
	return a.plugin.ValidateConfig(ctx, sdkprovider.ValidateConfigRequest{Config: tc, Stage: req.Stage})
}

func (a *Adapter) Doctor(ctx context.Context, req coreprovider.DoctorRequest) (*coreprovider.DoctorResult, error) {
	tc, err := sdkbridge.FromCoreConfig(req.Config)
	if err != nil {
		return nil, err
	}
	r, err := a.plugin.Doctor(ctx, sdkprovider.DoctorRequest{Config: tc, Stage: req.Stage})
	if err != nil {
		return nil, err
	}
	return &coreprovider.DoctorResult{Provider: r.Provider, Checks: r.Checks}, nil
}

func (a *Adapter) Plan(ctx context.Context, req coreprovider.PlanRequest) (*coreprovider.PlanResult, error) {
	tc, err := sdkbridge.FromCoreConfig(req.Config)
	if err != nil {
		return nil, err
	}
	r, err := a.plugin.Plan(ctx, sdkprovider.PlanRequest{Config: tc, Stage: req.Stage, Root: req.Root})
	if err != nil {
		return nil, err
	}
	// Re-hydrate Plan from any via JSON round-trip into *planner.Plan.
	var plan *planner.Plan
	if r.Plan != nil {
		b, marshalErr := json.Marshal(r.Plan)
		if marshalErr != nil {
			return nil, marshalErr
		}
		var p planner.Plan
		if err := json.Unmarshal(b, &p); err != nil {
			return nil, err
		}
		plan = &p
	}
	return &coreprovider.PlanResult{Provider: r.Provider, Plan: plan, Warnings: r.Warnings}, nil
}

func (a *Adapter) Deploy(ctx context.Context, req coreprovider.DeployRequest) (*coreprovider.DeployResult, error) {
	tc, err := sdkbridge.FromCoreConfig(req.Config)
	if err != nil {
		return nil, err
	}
	r, err := a.plugin.Deploy(ctx, sdkprovider.DeployRequest{Config: tc, Stage: req.Stage, Root: req.Root})
	if err != nil {
		return nil, err
	}
	out := &coreprovider.DeployResult{
		Provider:     r.Provider,
		DeploymentID: r.DeploymentID,
		Outputs:      r.Outputs,
		Metadata:     r.Metadata,
	}
	if len(r.Artifacts) > 0 {
		out.Artifacts = make([]coreprovider.Artifact, len(r.Artifacts))
		for i, art := range r.Artifacts {
			out.Artifacts[i] = coreprovider.Artifact{
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
		out.Functions = make(map[string]coreprovider.DeployedFunction, len(r.Functions))
		for k, f := range r.Functions {
			out.Functions[k] = coreprovider.DeployedFunction{
				ResourceName:       f.ResourceName,
				ResourceIdentifier: f.ResourceIdentifier,
				Metadata:           f.Metadata,
			}
		}
	}
	return out, nil
}

func (a *Adapter) Remove(ctx context.Context, req coreprovider.RemoveRequest) (*coreprovider.RemoveResult, error) {
	tc, err := sdkbridge.FromCoreConfig(req.Config)
	if err != nil {
		return nil, err
	}
	r, err := a.plugin.Remove(ctx, sdkprovider.RemoveRequest{
		Config: tc, Stage: req.Stage, Root: req.Root, Receipt: req.Receipt,
	})
	if err != nil {
		return nil, err
	}
	return &coreprovider.RemoveResult{Provider: r.Provider, Removed: r.Removed}, nil
}

func (a *Adapter) Invoke(ctx context.Context, req coreprovider.InvokeRequest) (*coreprovider.InvokeResult, error) {
	tc, err := sdkbridge.FromCoreConfig(req.Config)
	if err != nil {
		return nil, err
	}
	r, err := a.plugin.Invoke(ctx, sdkprovider.InvokeRequest{
		Config: tc, Stage: req.Stage, Function: req.Function, Payload: req.Payload,
	})
	if err != nil {
		return nil, err
	}
	return &coreprovider.InvokeResult{
		Provider: r.Provider, Function: r.Function, Output: r.Output,
		RunID: r.RunID, Workflow: r.Workflow,
	}, nil
}

func (a *Adapter) Logs(ctx context.Context, req coreprovider.LogsRequest) (*coreprovider.LogsResult, error) {
	tc, err := sdkbridge.FromCoreConfig(req.Config)
	if err != nil {
		return nil, err
	}
	r, err := a.plugin.Logs(ctx, sdkprovider.LogsRequest{
		Config: tc, Stage: req.Stage, Function: req.Function,
	})
	if err != nil {
		return nil, err
	}
	return &coreprovider.LogsResult{
		Provider: r.Provider, Function: r.Function, Lines: r.Lines, Workflow: r.Workflow,
	}, nil
}

// FetchMetrics satisfies core.ObservabilityCapable when the underlying plugin supports it.
func (a *Adapter) FetchMetrics(ctx context.Context, req coreprovider.MetricsRequest) (*coreprovider.MetricsResult, error) {
	obs, ok := a.plugin.(sdkprovider.ObservabilityCapable)
	if !ok {
		return &coreprovider.MetricsResult{Message: "provider does not support metrics"}, nil
	}
	tc, err := sdkbridge.FromCoreConfig(req.Config)
	if err != nil {
		return nil, err
	}
	r, err := obs.FetchMetrics(ctx, sdkprovider.MetricsRequest{Config: tc, Stage: req.Stage})
	if err != nil {
		return nil, err
	}
	return &coreprovider.MetricsResult{PerFunction: r.PerFunction, Message: r.Message}, nil
}

// FetchTraces satisfies core.ObservabilityCapable when the underlying plugin supports it.
func (a *Adapter) FetchTraces(ctx context.Context, req coreprovider.TracesRequest) (*coreprovider.TracesResult, error) {
	obs, ok := a.plugin.(sdkprovider.ObservabilityCapable)
	if !ok {
		return &coreprovider.TracesResult{Message: "provider does not support traces"}, nil
	}
	tc, err := sdkbridge.FromCoreConfig(req.Config)
	if err != nil {
		return nil, err
	}
	r, err := obs.FetchTraces(ctx, sdkprovider.TracesRequest{Config: tc, Stage: req.Stage})
	if err != nil {
		return nil, err
	}
	return &coreprovider.TracesResult{Traces: r.Traces, Message: r.Message}, nil
}

// PrepareDevStream satisfies core.DevStreamCapable.
// It first checks whether the underlying plugin implements localDevStreamCapable,
// which preserves the in-memory restore callback. Falls back to the serialisable
// sdkprovider.DevStreamCapable path when not available.
func (a *Adapter) PrepareDevStream(ctx context.Context, req coreprovider.DevStreamRequest) (*coreprovider.DevStreamSession, error) {
	tc, err := sdkbridge.FromCoreConfig(req.Config)
	if err != nil {
		return nil, err
	}
	treq := sdkprovider.DevStreamRequest{
		Config: tc, Stage: req.Stage, TunnelURL: req.TunnelURL, Region: req.Region,
	}
	if local, ok := a.plugin.(localDevStreamCapable); ok {
		r, localErr := local.PrepareDevStreamLocal(ctx, treq)
		if localErr != nil || r == nil {
			return nil, localErr
		}
		// Wrap the SDK session into a core session, preserving the restore callback.
		return coreprovider.NewDevStreamSession(r.EffectiveMode, r.MissingPrereqs, r.StatusMessage, r.Restore), nil
	}
	ds, ok := a.plugin.(sdkprovider.DevStreamCapable)
	if !ok {
		return nil, nil
	}
	r, err := ds.PrepareDevStream(ctx, treq)
	if err != nil || r == nil {
		return nil, err
	}
	return coreprovider.NewDevStreamSession(r.EffectiveMode, r.MissingPrereqs, r.StatusMessage, r.Restore), nil
}

// Recover satisfies core.RecoveryCapable when the underlying plugin supports it.
func (a *Adapter) Recover(ctx context.Context, req coreprovider.RecoveryRequest) (*coreprovider.RecoveryResult, error) {
	rc, ok := a.plugin.(sdkprovider.RecoveryCapable)
	if !ok {
		return &coreprovider.RecoveryResult{Recovered: false, Status: "unsupported"}, nil
	}
	tc, err := sdkbridge.FromCoreConfig(req.Config)
	if err != nil {
		return nil, err
	}
	r, err := rc.Recover(ctx, sdkprovider.RecoveryRequest{
		Config: tc, Root: req.Root, Service: req.Service,
		Stage: req.Stage, Region: req.Region, Mode: req.Mode, Journal: req.Journal,
	})
	if err != nil {
		return nil, err
	}
	return &coreprovider.RecoveryResult{
		Recovered: r.Recovered, Mode: r.Mode, Status: r.Status, Message: r.Message,
		Metadata: r.Metadata, Errors: r.Errors, ResumeData: r.ResumeData,
	}, nil
}

// SyncOrchestrations satisfies core.OrchestrationCapable when the underlying plugin supports it.
func (a *Adapter) SyncOrchestrations(ctx context.Context, req coreprovider.OrchestrationSyncRequest) (*coreprovider.OrchestrationSyncResult, error) {
	oc, ok := a.plugin.(sdkprovider.OrchestrationCapable)
	if !ok {
		return &coreprovider.OrchestrationSyncResult{}, nil
	}
	tc, err := sdkbridge.FromCoreConfig(req.Config)
	if err != nil {
		return nil, err
	}
	r, err := oc.SyncOrchestrations(ctx, sdkprovider.OrchestrationSyncRequest{
		Config: tc, Stage: req.Stage, Root: req.Root,
		FunctionResourceByName: req.FunctionResourceByName,
	})
	if err != nil {
		return nil, err
	}
	return &coreprovider.OrchestrationSyncResult{Metadata: r.Metadata, Outputs: r.Outputs}, nil
}

// RemoveOrchestrations satisfies core.OrchestrationCapable when the underlying plugin supports it.
func (a *Adapter) RemoveOrchestrations(ctx context.Context, req coreprovider.OrchestrationRemoveRequest) (*coreprovider.OrchestrationSyncResult, error) {
	oc, ok := a.plugin.(sdkprovider.OrchestrationCapable)
	if !ok {
		return &coreprovider.OrchestrationSyncResult{}, nil
	}
	tc, err := sdkbridge.FromCoreConfig(req.Config)
	if err != nil {
		return nil, err
	}
	r, err := oc.RemoveOrchestrations(ctx, sdkprovider.OrchestrationRemoveRequest{
		Config: tc, Stage: req.Stage, Root: req.Root,
	})
	if err != nil {
		return nil, err
	}
	return &coreprovider.OrchestrationSyncResult{Metadata: r.Metadata, Outputs: r.Outputs}, nil
}

// InvokeOrchestration satisfies core.OrchestrationCapable when the underlying plugin supports it.
func (a *Adapter) InvokeOrchestration(ctx context.Context, req coreprovider.OrchestrationInvokeRequest) (*coreprovider.InvokeResult, error) {
	oc, ok := a.plugin.(sdkprovider.OrchestrationCapable)
	if !ok {
		return nil, nil
	}
	tc, err := sdkbridge.FromCoreConfig(req.Config)
	if err != nil {
		return nil, err
	}
	r, err := oc.InvokeOrchestration(ctx, sdkprovider.OrchestrationInvokeRequest{
		Config: tc, Stage: req.Stage, Root: req.Root, Name: req.Name, Payload: req.Payload,
	})
	if err != nil {
		return nil, err
	}
	return &coreprovider.InvokeResult{
		Provider: r.Provider, Function: r.Function, Output: r.Output,
		RunID: r.RunID, Workflow: r.Workflow,
	}, nil
}

// InspectOrchestrations satisfies core.OrchestrationCapable when the underlying plugin supports it.
func (a *Adapter) InspectOrchestrations(ctx context.Context, req coreprovider.OrchestrationInspectRequest) (map[string]any, error) {
	oc, ok := a.plugin.(sdkprovider.OrchestrationCapable)
	if !ok {
		return nil, nil
	}
	tc, err := sdkbridge.FromCoreConfig(req.Config)
	if err != nil {
		return nil, err
	}
	return oc.InspectOrchestrations(ctx, sdkprovider.OrchestrationInspectRequest{
		Config: tc, Stage: req.Stage, Root: req.Root,
	})
}

package gcp

import (
	"context"
	"fmt"
	"strings"

	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	"github.com/runfabric/runfabric/platform/core/state/transactions"
	"github.com/runfabric/runfabric/platform/extensions/sdkbridge"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

func (p *Provider) FetchMetrics(ctx context.Context, req providers.MetricsRequest) (*providers.MetricsResult, error) {
	if req.Config == nil {
		return &providers.MetricsResult{Message: "GCP: use Cloud Console / Cloud Monitoring for function metrics."}, nil
	}
	metrics, err := FetchMetrics(ctx, req.Config, req.Stage)
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	for fn, m := range metrics {
		out[fn] = m
	}
	if len(out) == 0 {
		return &providers.MetricsResult{Message: "GCP: use Cloud Console / Cloud Monitoring for function metrics."}, nil
	}
	return &providers.MetricsResult{
		PerFunction: out,
		Message:     "GCP Cloud Monitoring metrics; use Cloud Console for more.",
	}, nil
}

func (p *Provider) FetchTraces(ctx context.Context, req providers.TracesRequest) (*providers.TracesResult, error) {
	if req.Config == nil {
		return &providers.TracesResult{Message: "GCP: use Cloud Console / Cloud Trace for traces."}, nil
	}
	summaries, err := FetchTraces(ctx, req.Config, req.Stage)
	if err != nil {
		return nil, err
	}
	traces := make([]any, 0, len(summaries))
	for _, summary := range summaries {
		traces = append(traces, summary)
	}
	if len(traces) == 0 {
		return &providers.TracesResult{Message: "GCP: use Cloud Console / Cloud Trace for traces."}, nil
	}
	return &providers.TracesResult{
		Traces:  traces,
		Message: "GCP Cloud Trace summaries; use Cloud Console for full details.",
	}, nil
}

func (p *Provider) PrepareDevStream(ctx context.Context, req providers.DevStreamRequest) (*providers.DevStreamSession, error) {
	sdkCfg, err := sdkbridge.FromCoreConfig(req.Config)
	if err != nil {
		return nil, err
	}
	state, err := RedirectToTunnel(ctx, sdkCfg, req.Stage, req.TunnelURL)
	if err != nil {
		return nil, err
	}
	if state == nil {
		return nil, nil
	}
	region := strings.TrimSpace(req.Region)
	if region == "" {
		region = strings.TrimSpace(providerRegion(sdkCfg))
	}
	return providers.NewDevStreamSession(
		string(state.EffectiveMode),
		state.MissingPrereqs,
		state.StatusMessage,
		func(restoreCtx context.Context) error {
			return state.Restore(restoreCtx, region)
		},
	), nil
}

func (p *Provider) SyncOrchestrations(ctx context.Context, req providers.OrchestrationSyncRequest) (*providers.OrchestrationSyncResult, error) {
	sdkCfg, err := sdkbridge.FromCoreConfig(req.Config)
	if err != nil {
		return nil, err
	}
	res, err := (Runner{}).SyncOrchestrations(ctx, sdkprovider.OrchestrationSyncRequest{
		Config:                 sdkCfg,
		Stage:                  req.Stage,
		Root:                   req.Root,
		FunctionResourceByName: req.FunctionResourceByName,
	})
	if err != nil {
		return nil, err
	}
	return &providers.OrchestrationSyncResult{Metadata: res.Metadata, Outputs: res.Outputs}, nil
}

func (p *Provider) RemoveOrchestrations(ctx context.Context, req providers.OrchestrationRemoveRequest) (*providers.OrchestrationSyncResult, error) {
	sdkCfg, err := sdkbridge.FromCoreConfig(req.Config)
	if err != nil {
		return nil, err
	}
	res, err := (Runner{}).RemoveOrchestrations(ctx, sdkprovider.OrchestrationRemoveRequest{
		Config: sdkCfg,
		Stage:  req.Stage,
		Root:   req.Root,
	})
	if err != nil {
		return nil, err
	}
	return &providers.OrchestrationSyncResult{Metadata: res.Metadata, Outputs: res.Outputs}, nil
}

func (p *Provider) InvokeOrchestration(ctx context.Context, req providers.OrchestrationInvokeRequest) (*providers.InvokeResult, error) {
	sdkCfg, err := sdkbridge.FromCoreConfig(req.Config)
	if err != nil {
		return nil, err
	}
	res, err := (Runner{}).InvokeOrchestration(ctx, sdkprovider.OrchestrationInvokeRequest{
		Config:  sdkCfg,
		Stage:   req.Stage,
		Root:    req.Root,
		Name:    req.Name,
		Payload: req.Payload,
	})
	if err != nil {
		return nil, err
	}
	return &providers.InvokeResult{
		Provider: res.Provider,
		Function: res.Function,
		Output:   res.Output,
		RunID:    res.RunID,
		Workflow: res.Workflow,
	}, nil
}

func (p *Provider) InspectOrchestrations(ctx context.Context, req providers.OrchestrationInspectRequest) (map[string]any, error) {
	sdkCfg, err := sdkbridge.FromCoreConfig(req.Config)
	if err != nil {
		return nil, err
	}
	return (Runner{}).InspectOrchestrations(ctx, sdkprovider.OrchestrationInspectRequest{
		Config: sdkCfg,
		Stage:  req.Stage,
		Root:   req.Root,
	})
}

func (p *Provider) Recover(ctx context.Context, req providers.RecoveryRequest) (*providers.RecoveryResult, error) {
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	metadata := map[string]string{
		"provider": "gcp-functions",
		"service":  req.Service,
		"stage":    req.Stage,
	}
	if journal, ok := req.Journal.(*transactions.JournalFile); ok && journal != nil {
		metadata["journalStatus"] = string(journal.Status)
		metadata["checkpoints"] = fmt.Sprintf("%d", len(journal.Checkpoints))
	}
	switch mode {
	case "rollback":
		return &providers.RecoveryResult{
			Recovered: false,
			Mode:      "rollback",
			Status:    "manual_action_required",
			Message:   "gcp rollback requires manual cleanup or remove/deploy rerun",
			Metadata:  metadata,
		}, nil
	case "resume":
		return &providers.RecoveryResult{
			Recovered: false,
			Mode:      "resume",
			Status:    "manual_action_required",
			Message:   "gcp resume is not automatic; run deploy again after inspecting state",
			Metadata:  metadata,
		}, nil
	case "inspect":
		return &providers.RecoveryResult{
			Recovered: false,
			Mode:      "inspect",
			Status:    "inspected",
			Message:   "gcp recovery inspect completed",
			Metadata:  metadata,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported recovery mode %q", req.Mode)
	}
}

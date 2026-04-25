package gcp

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

func (p *Provider) FetchMetrics(ctx context.Context, req sdkprovider.MetricsRequest) (*sdkprovider.MetricsResult, error) {
	if req.Config == nil {
		return &sdkprovider.MetricsResult{Message: "GCP: use Cloud Console / Cloud Monitoring for function metrics."}, nil
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
		return &sdkprovider.MetricsResult{Message: "GCP: use Cloud Console / Cloud Monitoring for function metrics."}, nil
	}
	return &sdkprovider.MetricsResult{
		PerFunction: out,
		Message:     "GCP Cloud Monitoring metrics; use Cloud Console for more.",
	}, nil
}

func (p *Provider) FetchTraces(ctx context.Context, req sdkprovider.TracesRequest) (*sdkprovider.TracesResult, error) {
	if req.Config == nil {
		return &sdkprovider.TracesResult{Message: "GCP: use Cloud Console / Cloud Trace for traces."}, nil
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
		return &sdkprovider.TracesResult{Message: "GCP: use Cloud Console / Cloud Trace for traces."}, nil
	}
	return &sdkprovider.TracesResult{
		Traces:  traces,
		Message: "GCP Cloud Trace summaries; use Cloud Console for full details.",
	}, nil
}

func (p *Provider) PrepareDevStream(ctx context.Context, req sdkprovider.DevStreamRequest) (*sdkprovider.DevStreamSession, error) {
	return sdkprovider.PrepareLifecycleDevStream(ProviderID, req.Config, req.Stage, req.TunnelURL)
}

func (p *Provider) SyncOrchestrations(ctx context.Context, req sdkprovider.OrchestrationSyncRequest) (*sdkprovider.OrchestrationSyncResult, error) {
	return (Runner{}).SyncOrchestrations(ctx, req)
}

func (p *Provider) RemoveOrchestrations(ctx context.Context, req sdkprovider.OrchestrationRemoveRequest) (*sdkprovider.OrchestrationSyncResult, error) {
	return (Runner{}).RemoveOrchestrations(ctx, req)
}

func (p *Provider) InvokeOrchestration(ctx context.Context, req sdkprovider.OrchestrationInvokeRequest) (*sdkprovider.InvokeResult, error) {
	return (Runner{}).InvokeOrchestration(ctx, req)
}

func (p *Provider) InspectOrchestrations(ctx context.Context, req sdkprovider.OrchestrationInspectRequest) (map[string]any, error) {
	return (Runner{}).InspectOrchestrations(ctx, req)
}

func (p *Provider) Recover(ctx context.Context, req sdkprovider.RecoveryRequest) (*sdkprovider.RecoveryResult, error) {
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	metadata := map[string]string{
		"provider": "gcp-functions",
		"service":  req.Service,
		"stage":    req.Stage,
	}
	if req.Journal != nil {
		v := reflect.ValueOf(req.Journal)
		if v.Kind() == reflect.Pointer && !v.IsNil() {
			e := v.Elem()
			if e.Kind() == reflect.Struct {
				if s := e.FieldByName("Status"); s.IsValid() {
					metadata["journalStatus"] = fmt.Sprint(s.Interface())
				}
				if c := e.FieldByName("Checkpoints"); c.IsValid() && c.Kind() == reflect.Slice {
					metadata["checkpoints"] = fmt.Sprintf("%d", c.Len())
				}
			}
		}
	}
	switch mode {
	case "rollback":
		if req.Config == nil {
			return nil, fmt.Errorf("config required for rollback")
		}
		if _, err := (Remover{}).Remove(ctx, req.Config, req.Stage, req.Root, nil); err != nil {
			return nil, err
		}
		metadata["strategy"] = "remove"
		return &sdkprovider.RecoveryResult{
			Recovered: true,
			Mode:      "rollback",
			Status:    "rolled_back_via_remove",
			Message:   "gcp rollback completed by running provider remove",
			Metadata:  metadata,
		}, nil
	case "resume":
		if req.Config == nil {
			return nil, fmt.Errorf("config required for resume")
		}
		if _, err := (Runner{}).Deploy(ctx, req.Config, req.Stage, req.Root); err != nil {
			return nil, err
		}
		metadata["strategy"] = "deploy"
		return &sdkprovider.RecoveryResult{
			Recovered: true,
			Mode:      "resume",
			Status:    "resumed_via_deploy",
			Message:   "gcp resume completed by running provider deploy",
			Metadata:  metadata,
		}, nil
	case "inspect":
		return &sdkprovider.RecoveryResult{
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

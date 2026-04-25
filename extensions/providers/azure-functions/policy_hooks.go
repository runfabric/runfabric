package azure

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

func PrepareDevStreamPolicy(ctx context.Context, cfg sdkprovider.Config, stage, tunnelURL string) (*sdkprovider.DevStreamSession, error) {
	return sdkprovider.PrepareLifecycleDevStream("azure-functions", cfg, stage, tunnelURL)
}

func FetchMetricsPolicy(ctx context.Context, cfg sdkprovider.Config, stage string) (*sdkprovider.MetricsResult, error) {
	perFn, err := FetchMetrics(ctx, cfg, stage)
	if err != nil {
		return nil, err
	}
	out := make(map[string]any, len(perFn))
	for fn, m := range perFn {
		out[fn] = m
	}
	if len(out) == 0 {
		return &sdkprovider.MetricsResult{Message: "Azure: use Azure Portal / Application Insights for function metrics."}, nil
	}
	return &sdkprovider.MetricsResult{PerFunction: out, Message: "Azure Application Insights metrics; use Azure Portal for more."}, nil
}

func FetchTracesPolicy(ctx context.Context, cfg sdkprovider.Config, stage string) (*sdkprovider.TracesResult, error) {
	summaries, err := FetchTraces(ctx, cfg, stage)
	if err != nil {
		return nil, err
	}
	traces := make([]any, 0, len(summaries))
	for _, s := range summaries {
		traces = append(traces, s)
	}
	if len(traces) == 0 {
		return &sdkprovider.TracesResult{Message: "Azure: use Azure Portal / Application Insights for traces."}, nil
	}
	return &sdkprovider.TracesResult{Traces: traces, Message: "Azure Application Insights traces; use Azure Portal for full details."}, nil
}

func RecoverPolicy(ctx context.Context, req sdkprovider.RecoveryRequest) (*sdkprovider.RecoveryResult, error) {
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	switch mode {
	case "rollback":
		if req.Config == nil {
			return nil, fmt.Errorf("config required for rollback")
		}
		if _, err := (Remover{}).Remove(ctx, req.Config, req.Stage, req.Root, nil); err != nil {
			return nil, err
		}
		return &sdkprovider.RecoveryResult{
			Recovered: true,
			Mode:      "rollback",
			Status:    "rolled_back_via_remove",
			Message:   "azure rollback completed by running provider remove",
			Metadata: map[string]string{
				"provider": "azure-functions",
				"service":  req.Service,
				"stage":    req.Stage,
				"strategy": "remove",
			},
		}, nil
	case "resume":
		if req.Config == nil {
			return nil, fmt.Errorf("config required for resume")
		}
		if _, err := (Runner{}).Deploy(ctx, req.Config, req.Stage, req.Root); err != nil {
			return nil, err
		}
		return &sdkprovider.RecoveryResult{
			Recovered: true,
			Mode:      "resume",
			Status:    "resumed_via_deploy",
			Message:   "azure resume completed by running provider deploy",
			Metadata: map[string]string{
				"provider": "azure-functions",
				"service":  req.Service,
				"stage":    req.Stage,
				"strategy": "deploy",
			},
		}, nil
	case "inspect":
		metadata := map[string]string{
			"provider": "azure-functions",
			"service":  req.Service,
			"stage":    req.Stage,
		}
		if status, checkpoints := describeJournal(req.Journal); status != "" || checkpoints != "" {
			if status != "" {
				metadata["journalStatus"] = status
			}
			if checkpoints != "" {
				metadata["checkpoints"] = checkpoints
			}
		}
		return &sdkprovider.RecoveryResult{
			Recovered: false,
			Mode:      "inspect",
			Status:    "inspected",
			Message:   "azure recovery inspect completed",
			Metadata:  metadata,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported recovery mode %q", req.Mode)
	}
}

func describeJournal(journal any) (string, string) {
	if journal == nil {
		return "", ""
	}
	v := reflect.ValueOf(journal)
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return "", ""
		}
		v = v.Elem()
	}
	if !v.IsValid() || v.Kind() != reflect.Struct {
		return "", ""
	}
	status := ""
	if field := v.FieldByName("Status"); field.IsValid() {
		status = strings.TrimSpace(fmt.Sprint(field.Interface()))
	}
	checkpoints := ""
	if field := v.FieldByName("Checkpoints"); field.IsValid() && (field.Kind() == reflect.Slice || field.Kind() == reflect.Array) {
		checkpoints = fmt.Sprintf("%d", field.Len())
	}
	return status, checkpoints
}

func SyncOrchestrationsPolicy(ctx context.Context, req sdkprovider.OrchestrationSyncRequest) (*sdkprovider.OrchestrationSyncResult, error) {
	return (Runner{}).SyncOrchestrations(ctx, req)
}

func RemoveOrchestrationsPolicy(ctx context.Context, req sdkprovider.OrchestrationRemoveRequest) (*sdkprovider.OrchestrationSyncResult, error) {
	return (Runner{}).RemoveOrchestrations(ctx, req)
}

func InvokeOrchestrationPolicy(ctx context.Context, req sdkprovider.OrchestrationInvokeRequest) (*sdkprovider.InvokeResult, error) {
	return (Runner{}).InvokeOrchestration(ctx, req)
}

func InspectOrchestrationsPolicy(ctx context.Context, req sdkprovider.OrchestrationInspectRequest) (map[string]any, error) {
	return (Runner{}).InspectOrchestrations(ctx, req)
}

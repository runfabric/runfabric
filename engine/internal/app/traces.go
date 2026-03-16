package app

import (
	"context"

	awsprovider "github.com/runfabric/runfabric/engine/providers/aws"
	azureprovider "github.com/runfabric/runfabric/engine/providers/azure"
	gcpprovider "github.com/runfabric/runfabric/engine/providers/gcp"
)

// Traces returns trace data for the deployed service (from receipt/metadata or provider).
// When all is true, output is aggregated by service/stage. For AWS, fetches X-Ray trace summaries when available.
func Traces(configPath, stage, providerOverride string, all bool) (any, error) {
	ctx, err := Bootstrap(configPath, stage, providerOverride)
	if err != nil {
		return nil, err
	}
	receipt, _ := ctx.Backends.Receipts.Load(ctx.Stage)
	out := map[string]any{
		"provider": ctx.Config.Provider.Name,
		"stage":    ctx.Stage,
		"service":  ctx.Config.Service,
		"traces":   []any{},
	}
	if all {
		out["aggregated"] = "by service/stage"
		out["functionCount"] = len(ctx.Config.Functions)
	}
	if receipt != nil {
		out["deploymentId"] = receipt.DeploymentID
		out["updatedAt"] = receipt.UpdatedAt
	}
	switch ctx.Config.Provider.Name {
	case "aws", "aws-lambda":
		summaries, err := awsprovider.FetchXRayTraces(context.Background(), ctx.Config, ctx.Stage)
		if err == nil {
			out["traces"] = summaries
			out["message"] = "X-Ray trace summaries (last 1h); use AWS console for full trace details."
		} else {
			out["message"] = "Traces: use provider console or runfabric logs when X-Ray is unavailable."
		}
	case "gcp":
		summaries, err := gcpprovider.FetchTraces(context.Background(), ctx.Config, ctx.Stage)
		if err == nil && len(summaries) > 0 {
			out["traces"] = summaries
			out["message"] = "GCP Cloud Trace summaries; use Cloud Console for full details."
		} else {
			out["message"] = "GCP: use Cloud Console / Cloud Trace for traces."
		}
	case "azure":
		summaries, err := azureprovider.FetchTraces(context.Background(), ctx.Config, ctx.Stage)
		if err == nil && len(summaries) > 0 {
			out["traces"] = summaries
			out["message"] = "Azure Application Insights traces; use Azure Portal for full details."
		} else {
			out["message"] = "Azure: use Azure Portal / Application Insights for traces."
		}
	default:
		out["message"] = "Traces: use provider console or runfabric logs for now; trace export coming soon."
	}
	return out, nil
}

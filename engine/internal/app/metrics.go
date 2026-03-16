package app

import (
	"context"

	awsprovider "github.com/runfabric/runfabric/engine/providers/aws"
	azureprovider "github.com/runfabric/runfabric/engine/providers/azure"
	gcpprovider "github.com/runfabric/runfabric/engine/providers/gcp"
)

// Metrics returns metrics for the deployed service (from receipt/metadata or provider).
// When all is true, output is aggregated by service/stage. For AWS Lambda, fetches CloudWatch metrics (Invocations, Errors, Duration) when available.
func Metrics(configPath, stage, providerOverride string, all bool) (any, error) {
	ctx, err := Bootstrap(configPath, stage, providerOverride)
	if err != nil {
		return nil, err
	}
	receipt, _ := ctx.Backends.Receipts.Load(ctx.Stage)
	out := map[string]any{
		"provider": ctx.Config.Provider.Name,
		"stage":    ctx.Stage,
		"service":  ctx.Config.Service,
		"metrics":  map[string]any{"invocations": nil, "errors": nil, "duration": nil},
	}
	if all {
		out["aggregated"] = "by service/stage"
		out["functionCount"] = len(ctx.Config.Functions)
	}
	if receipt != nil {
		out["deploymentId"] = receipt.DeploymentID
		out["functionCount"] = len(receipt.Functions)
	}
	// Provider-specific metrics.
	switch ctx.Config.Provider.Name {
	case "aws", "aws-lambda":
		cloudMetrics, err := awsprovider.FetchLambdaMetrics(context.Background(), ctx.Config, ctx.Stage)
		if err == nil && len(cloudMetrics) > 0 {
			out["perFunction"] = cloudMetrics
			out["message"] = "CloudWatch metrics (last 1h); use provider console for more."
		} else {
			out["message"] = "Metrics: use provider console (e.g. CloudWatch) when not deployed or region unavailable."
		}
	case "gcp":
		perFn, err := gcpprovider.FetchMetrics(context.Background(), ctx.Config, ctx.Stage)
		if err == nil && len(perFn) > 0 {
			out["perFunction"] = perFn
			out["message"] = "GCP Cloud Monitoring metrics; use Cloud Console for more."
		} else {
			out["message"] = "GCP: use Cloud Console / Cloud Monitoring for function metrics."
		}
	case "azure":
		perFn, err := azureprovider.FetchMetrics(context.Background(), ctx.Config, ctx.Stage)
		if err == nil && len(perFn) > 0 {
			out["perFunction"] = perFn
			out["message"] = "Azure Application Insights metrics; use Azure Portal for more."
		} else {
			out["message"] = "Azure: use Azure Portal / Application Insights for function metrics."
		}
	default:
		out["message"] = "Metrics: use provider console (e.g. CloudWatch) for now; metrics export coming soon."
	}
	return out, nil
}

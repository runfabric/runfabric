package azure

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// FunctionMetrics holds per-function metrics (compatible shape with AWS Lambda metrics for CLI output).
type FunctionMetrics struct {
	Invocations *float64 `json:"invocations,omitempty"`
	Errors      *float64 `json:"errors,omitempty"`
	DurationAvg *float64 `json:"durationAvgMs,omitempty"`
}

// FetchMetrics returns per-function metrics from Azure Log Analytics/App Insights over the last hour.
func FetchMetrics(ctx context.Context, cfg sdkprovider.Config, stage string) (map[string]FunctionMetrics, error) {
	workspaceID := strings.TrimSpace(sdkprovider.Env("AZURE_LOG_ANALYTICS_WORKSPACE_ID"))
	token := strings.TrimSpace(sdkprovider.Env("AZURE_ACCESS_TOKEN"))
	if workspaceID == "" || token == "" {
		return map[string]FunctionMetrics{}, nil
	}

	serviceName := sdkprovider.Service(cfg)
	if serviceName == "" {
		serviceName = "service"
	}
	appName := fmt.Sprintf("%s-%s", serviceName, stage)
	query := fmt.Sprintf(`AppRequests
| where TimeGenerated > ago(1h)
| where cloud_RoleName =~ %q
| extend function_name=tostring(split(operation_Name, '/')[0])
| summarize invocations=count(), errors=countif(success == false), durationAvgMs=avg(todouble(column_ifexists("DurationMs", real(null)))) by function_name`, appName)

	rows, err := azureQueryRows(ctx, workspaceID, query)
	if err != nil {
		return nil, err
	}

	out := make(map[string]FunctionMetrics)
	for _, row := range rows {
		if len(row) < 4 {
			continue
		}
		fn := strings.TrimSpace(anyToString(row[0]))
		if fn == "" {
			continue
		}
		inv := anyToFloat64Ptr(row[1])
		errCount := anyToFloat64Ptr(row[2])
		dur := anyToFloat64Ptr(row[3])
		out[fn] = FunctionMetrics{Invocations: inv, Errors: errCount, DurationAvg: dur}
	}
	return out, nil
}

func azureQueryRows(ctx context.Context, workspaceID, query string) ([][]any, error) {
	reqURL := fmt.Sprintf("https://api.loganalytics.io/v1/workspaces/%s/query?query=%s", workspaceID, url.QueryEscape(query))
	var payload struct {
		Tables []struct {
			Rows [][]any `json:"rows"`
		} `json:"tables"`
	}
	if err := sdkprovider.APIGet(ctx, reqURL, "AZURE_ACCESS_TOKEN", &payload); err != nil {
		return nil, err
	}
	if len(payload.Tables) == 0 {
		return [][]any{}, nil
	}
	return payload.Tables[0].Rows, nil
}

func anyToString(v any) string {
	if v == nil {
		return ""
	}
	s, ok := v.(string)
	if ok {
		return s
	}
	return fmt.Sprint(v)
}

func anyToFloat64Ptr(v any) *float64 {
	s := strings.TrimSpace(anyToString(v))
	if s == "" || strings.EqualFold(s, "null") {
		return nil
	}
	var f float64
	if _, err := fmt.Sscanf(s, "%f", &f); err != nil {
		return nil
	}
	return &f
}

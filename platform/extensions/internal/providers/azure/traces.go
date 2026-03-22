package azure

import (
	"context"
	"fmt"
	"strings"

	"github.com/runfabric/runfabric/platform/core/model/config"
	"github.com/runfabric/runfabric/platform/deploy/apiutil"
)

// TraceSummary is a simplified trace summary for CLI output (compatible shape with AWS X-Ray).
type TraceSummary struct {
	ID           string  `json:"id,omitempty"`
	Duration     float64 `json:"duration,omitempty"`
	ResponseTime float64 `json:"responseTime,omitempty"`
	HTTPStatus   *int32  `json:"httpStatus,omitempty"`
	ServiceCount *int    `json:"serviceCount,omitempty"`
	HasError     *bool   `json:"hasError,omitempty"`
}

// FetchTraces returns trace summaries from Azure Log Analytics/App Insights over the last hour.
func FetchTraces(ctx context.Context, cfg *config.Config, stage string) ([]TraceSummary, error) {
	workspaceID := strings.TrimSpace(apiutil.Env("AZURE_LOG_ANALYTICS_WORKSPACE_ID"))
	if workspaceID == "" || strings.TrimSpace(apiutil.Env("AZURE_ACCESS_TOKEN")) == "" {
		return []TraceSummary{}, nil
	}

	appName := fmt.Sprintf("%s-%s", cfg.Service, stage)
	query := fmt.Sprintf(`AppRequests
| where TimeGenerated > ago(1h)
| where cloud_RoleName =~ %q
| top 50 by TimeGenerated desc
| project id, durationMs=todouble(column_ifexists("DurationMs", real(0))), resultCode=tostring(column_ifexists("ResultCode", "")), success=tobool(column_ifexists("Success", true))`, appName)

	rows, err := azureQueryRows(ctx, workspaceID, query)
	if err != nil {
		return nil, err
	}

	out := make([]TraceSummary, 0, len(rows))
	for _, row := range rows {
		if len(row) < 4 {
			continue
		}
		id := anyToString(row[0])
		durMs := anyToFloat64Ptr(row[1])
		code := strings.TrimSpace(anyToString(row[2]))
		successVal := strings.EqualFold(strings.TrimSpace(anyToString(row[3])), "true")

		var durSeconds float64
		if durMs != nil {
			durSeconds = *durMs / 1000.0
		}
		hasError := !successVal
		svcCount := 1
		var httpStatus *int32
		if code != "" {
			var parsed int32
			if _, parseErr := fmt.Sscanf(code, "%d", &parsed); parseErr == nil {
				httpStatus = &parsed
				if parsed >= 500 {
					hasError = true
				}
			}
		}

		out = append(out, TraceSummary{
			ID:           id,
			Duration:     durSeconds,
			ResponseTime: durSeconds,
			HTTPStatus:   httpStatus,
			ServiceCount: &svcCount,
			HasError:     &hasError,
		})
	}
	return out, nil
}

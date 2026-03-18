package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/runfabric/runfabric/engine/internal/apiutil"
	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/extensions/providers"
	"github.com/runfabric/runfabric/engine/internal/state"
)

// Logger fetches recent logs from Azure (Log Analytics if workspace set, else portal link).
type Logger struct{}

func (Logger) Logs(ctx context.Context, cfg *config.Config, stage, function string, receipt *state.Receipt) (*providers.LogsResult, error) {
	appName := receipt.Outputs["app_name"]
	if appName == "" {
		appName = fmt.Sprintf("%s-%s", cfg.Service, stage)
	}
	workspaceID := apiutil.Env("AZURE_LOG_ANALYTICS_WORKSPACE_ID")
	if workspaceID != "" && apiutil.Env("AZURE_ACCESS_TOKEN") != "" {
		lines, err := queryLogAnalytics(ctx, receipt, appName, workspaceID)
		if err == nil && len(lines) > 0 {
			return &providers.LogsResult{Provider: "azure-functions", Function: function, Lines: lines}, nil
		}
	}
	subID := apiutil.Env("AZURE_SUBSCRIPTION_ID")
	rg := receipt.Outputs["resource_group"]
	var portalLink string
	if subID != "" && rg != "" {
		portalLink = fmt.Sprintf("https://portal.azure.com/#@/resource/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Web/sites/%s/logStream", subID, rg, appName)
	} else {
		portalLink = fmt.Sprintf("https://portal.azure.com (Function App: %s → Log stream)", appName)
	}
	return &providers.LogsResult{
		Provider: "azure-functions",
		Function: function,
		Lines:    []string{"View logs: " + portalLink, "Or set AZURE_LOG_ANALYTICS_WORKSPACE_ID for CLI log fetch."},
	}, nil
}

func queryLogAnalytics(ctx context.Context, receipt *state.Receipt, appName, workspaceID string) ([]string, error) {
	token := apiutil.Env("AZURE_ACCESS_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("AZURE_ACCESS_TOKEN required")
	}
	// Log Analytics query: recent traces/logs for the function app (AppServiceConsoleLogs or similar)
	query := url.QueryEscape("AppServiceConsoleLogs | where TimeGenerated > ago(1h) | order by TimeGenerated desc | take 50 | project TimeGenerated, ResultMessage")
	reqURL := fmt.Sprintf("https://api.loganalytics.io/v1/workspaces/%s/query?query=%s", workspaceID, query)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := apiutil.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("log analytics: %s", resp.Status)
	}
	b, _ := io.ReadAll(resp.Body)
	var result struct {
		Tables []struct {
			Rows [][]interface{} `json:"rows"`
		} `json:"tables"`
	}
	if json.Unmarshal(b, &result) != nil || len(result.Tables) == 0 {
		return nil, fmt.Errorf("no tables")
	}
	var lines []string
	for _, row := range result.Tables[0].Rows {
		if len(row) >= 2 {
			ts, _ := row[0].(string)
			msg, _ := row[1].(string)
			lines = append(lines, ts+" "+msg)
		}
	}
	return lines, nil
}

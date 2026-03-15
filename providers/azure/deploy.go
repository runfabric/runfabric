// Package azure implements Azure Functions REST API deploy/remove/invoke/logs.
package azure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/runfabric/runfabric/internal/apiutil"
	"github.com/runfabric/runfabric/internal/config"
	"github.com/runfabric/runfabric/internal/providers"
)

// Runner deploys by creating resource group and function app via Azure Management REST API.
type Runner struct{}

func (Runner) Deploy(ctx context.Context, cfg *config.Config, stage, root string) (*providers.DeployResult, error) {
	if apiutil.Env("AZURE_ACCESS_TOKEN") == "" {
		return nil, fmt.Errorf("AZURE_ACCESS_TOKEN is required (e.g. from az account get-access-token --resource https://management.azure.com)")
	}
	if apiutil.Env("AZURE_SUBSCRIPTION_ID") == "" {
		return nil, fmt.Errorf("AZURE_SUBSCRIPTION_ID is required")
	}
	rg := apiutil.Env("AZURE_RESOURCE_GROUP")
	if rg == "" {
		rg = cfg.Service + "-" + stage
	}
	appName := fmt.Sprintf("%s-%s", cfg.Service, stage)
	subID := apiutil.Env("AZURE_SUBSCRIPTION_ID")
	base := "https://management.azure.com/subscriptions/" + subID
	rgURL := base + "/resourcegroups/" + rg + "?api-version=2021-04-01"
	rgBody := map[string]any{"location": "westus2"}
	bodyBytes, _ := json.Marshal(rgBody)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPut, rgURL, bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer "+apiutil.Env("AZURE_ACCESS_TOKEN"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := apiutil.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("azure resource group: %s: %s", resp.Status, string(b))
	}
	appURL := base + "/resourceGroups/" + rg + "/providers/Microsoft.Web/sites/" + appName + "?api-version=2022-03-01"
	appBody := map[string]any{
		"location": "westus2",
		"kind":     "functionapp",
		"properties": map[string]any{
			"reserved": true,
			"siteConfig": map[string]any{"linuxFxVersion": "NODE|20"},
		},
	}
	bodyBytes, _ = json.Marshal(appBody)
	req, _ = http.NewRequestWithContext(ctx, http.MethodPut, appURL, bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer "+apiutil.Env("AZURE_ACCESS_TOKEN"))
	req.Header.Set("Content-Type", "application/json")
	resp, err = apiutil.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("azure function app: %s: %s", resp.Status, string(body))
	}
	result := apiutil.BuildDeployResult("azure-functions", cfg, stage)
	result.Outputs["resource_group"] = rg
	result.Outputs["app_name"] = appName
	result.Outputs["url"] = "https://" + appName + ".azurewebsites.net"
	result.Metadata["app"] = appName
	return result, nil
}

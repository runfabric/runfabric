// Package azure implements Azure Functions REST API deploy/remove/invoke/logs.
package azure

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// Runner deploys by creating resource group and function app via Azure Management REST API.
type Runner struct{}

func (Runner) Deploy(ctx context.Context, cfg sdkprovider.Config, stage, root string) (*sdkprovider.DeployResult, error) {
	serviceName := sdkprovider.Service(cfg)
	if serviceName == "" {
		serviceName = "service"
	}
	functions := sdkprovider.Functions(cfg)
	_ = root
	if sdkprovider.Env("AZURE_ACCESS_TOKEN") == "" {
		return nil, fmt.Errorf("AZURE_ACCESS_TOKEN is required (e.g. from az account get-access-token --resource https://management.azure.com)")
	}
	if sdkprovider.Env("AZURE_SUBSCRIPTION_ID") == "" {
		return nil, fmt.Errorf("AZURE_SUBSCRIPTION_ID is required")
	}
	rg := sdkprovider.Env("AZURE_RESOURCE_GROUP")
	if rg == "" {
		rg = serviceName + "-" + stage
	}
	appName := fmt.Sprintf("%s-%s", serviceName, stage)
	subID := sdkprovider.Env("AZURE_SUBSCRIPTION_ID")
	base := azureManagementBase() + "/subscriptions/" + subID
	rgURL := base + "/resourcegroups/" + rg + "?api-version=2021-04-01"
	rgBody := map[string]any{"location": "westus2"}
	bodyBytes, _ := json.Marshal(rgBody)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPut, rgURL, bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer "+sdkprovider.Env("AZURE_ACCESS_TOKEN"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := sdkprovider.DefaultClient.Do(req)
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
			"reserved":   true,
			"siteConfig": map[string]any{"linuxFxVersion": "NODE|20"},
		},
	}
	bodyBytes, _ = json.Marshal(appBody)
	req, _ = http.NewRequestWithContext(ctx, http.MethodPut, appURL, bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer "+sdkprovider.Env("AZURE_ACCESS_TOKEN"))
	req.Header.Set("Content-Type", "application/json")
	resp, err = sdkprovider.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	asyncOpURL := resp.Header.Get("Azure-AsyncOperation")
	if asyncOpURL == "" {
		asyncOpURL = resp.Header.Get("Location")
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("azure function app: %s: %s", resp.Status, string(body))
	}
	// ARM PUT is async — wait until the app is Running before returning.
	siteURL := appURL
	if err := waitUntilAppReady(ctx, asyncOpURL, siteURL); err != nil {
		return nil, fmt.Errorf("wait for function app %s: %w", appName, err)
	}
	// Push the function code so the app actually serves the handlers. Real Azure
	// uses Kudu zip deploy; this is best-effort — if the SCM endpoint is absent
	// (e.g. an emulator that does not implement Kudu) the app shell is still
	// deployed, so we record the outcome instead of failing the deploy.
	result := sdkprovider.BuildDeployResult("azure-functions", cfg, stage)
	result.Outputs["code_deploy"] = deployFunctionCode(ctx, appName, root)

	appResourceID := "/subscriptions/" + subID + "/resourceGroups/" + rg + "/providers/Microsoft.Web/sites/" + appName
	appHost := azureAppHost(appName)
	result.Outputs["resource_group"] = rg
	result.Outputs["app_name"] = appName
	result.Outputs["url"] = appHost
	result.Metadata["app"] = appName
	for fnName := range functions {
		// HTTP-triggered Azure Functions are served at <appHost>/api/<function>.
		fnURL := appHost + "/api/" + fnName
		deployed := result.Functions[fnName]
		deployed.ResourceName = appName + "/" + fnName
		deployed.ResourceIdentifier = appResourceID
		deployed.Metadata = map[string]string{
			"appName":       appName,
			"resourceGroup": rg,
			"resourceId":    appResourceID,
			"url":           fnURL,
		}
		result.Functions[fnName] = deployed
		result.Outputs["url_"+fnName] = fnURL
	}
	return result, nil
}

// deployFunctionCode zips the project root and pushes it to the function app via
// Kudu zip deploy (POST <scm>/api/zipdeploy). It is best-effort and returns a
// short status string ("deployed", "skipped: …", "failed: …") recorded in the
// deploy outputs rather than an error, so a missing/limited SCM endpoint does not
// abort the overall deploy.
func deployFunctionCode(ctx context.Context, appName, root string) string {
	if strings.TrimSpace(root) == "" {
		return "skipped: no source root"
	}
	zipBytes, err := zipDir(root)
	if err != nil {
		return "skipped: zip source: " + err.Error()
	}
	if len(zipBytes) == 0 {
		return "skipped: empty source"
	}
	url := azureScmHost(appName) + "/api/zipdeploy"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(zipBytes))
	if err != nil {
		return "skipped: build request: " + err.Error()
	}
	if token := sdkprovider.Env("AZURE_ACCESS_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/zip")
	resp, err := sdkprovider.DefaultClient.Do(req)
	if err != nil {
		return "failed: " + err.Error()
	}
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return "deployed"
	}
	return fmt.Sprintf("skipped: zipdeploy returned %s", resp.Status)
}

// zipDir builds a zip archive of dir, excluding node_modules and .git, matching
// the layout Kudu zip deploy expects (paths relative to the app root).
func zipDir(dir string) ([]byte, error) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == "node_modules" || info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.Contains(path, "node_modules"+string(filepath.Separator)) || strings.Contains(path, ".git"+string(filepath.Separator)) {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return nil
		}
		f, err := w.Create(rel)
		if err != nil {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, err = f.Write(body)
		return err
	})
	if err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

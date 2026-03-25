package azure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/runfabric/runfabric/platform/deploy/apiutil"
	"github.com/runfabric/runfabric/platform/extensions/sdkbridge"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

var azureManagementAPI = "https://management.azure.com"

func (Runner) SyncOrchestrations(ctx context.Context, req sdkprovider.OrchestrationSyncRequest) (*sdkprovider.OrchestrationSyncResult, error) {
	decls, err := durableFunctionsFromConfig(req.Config)
	if err != nil {
		return nil, err
	}
	if len(decls) == 0 {
		return &sdkprovider.OrchestrationSyncResult{}, nil
	}
	rg, appName := azureDeploymentContext(req.Config, req.Stage, req.Root)
	settingsOutcome, err := azureSyncDurableAppSettings(ctx, req.Config, rg, appName, decls)
	if err != nil {
		return nil, err
	}
	res := &sdkprovider.OrchestrationSyncResult{Metadata: map[string]string{}, Outputs: map[string]string{}}
	for _, decl := range decls {
		operation := settingsOutcome[decl.Name]
		if operation == "" {
			operation = "linked"
		}
		res.Metadata["durable:"+decl.Name+":operation"] = operation
		res.Metadata["durable:"+decl.Name+":orchestrator"] = decl.Orchestrator
		res.Metadata["durable:"+decl.Name+":app"] = appName
		res.Metadata["durable:"+decl.Name+":resourceGroup"] = rg
		res.Metadata["durable:"+decl.Name+":console"] = azureFunctionAppConsoleLink(req.Config, rg, appName)
		if strings.TrimSpace(decl.TaskHub) != "" {
			res.Metadata["durable:"+decl.Name+":taskHub"] = decl.TaskHub
		}
		if strings.TrimSpace(decl.StorageConnectionSetting) != "" {
			res.Metadata["durable:"+decl.Name+":storageConnectionSetting"] = decl.StorageConnectionSetting
		}
	}
	return res, nil
}

func (Runner) RemoveOrchestrations(ctx context.Context, req sdkprovider.OrchestrationRemoveRequest) (*sdkprovider.OrchestrationSyncResult, error) {
	decls, err := durableFunctionsFromConfig(req.Config)
	if err != nil {
		return nil, err
	}
	if len(decls) == 0 {
		return &sdkprovider.OrchestrationSyncResult{}, nil
	}
	rg, appName := azureDeploymentContext(req.Config, req.Stage, req.Root)
	settingsOutcome, err := azureRemoveDurableAppSettings(ctx, req.Config, rg, appName, decls)
	if err != nil {
		return nil, err
	}
	res := &sdkprovider.OrchestrationSyncResult{Metadata: map[string]string{}, Outputs: map[string]string{}}
	for _, decl := range decls {
		operation := settingsOutcome[decl.Name]
		if operation == "" {
			operation = "removed-with-app"
		}
		res.Metadata["durable:"+decl.Name+":operation"] = operation
	}
	return res, nil
}

func (Runner) InvokeOrchestration(ctx context.Context, req sdkprovider.OrchestrationInvokeRequest) (*sdkprovider.InvokeResult, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("orchestration name is required")
	}
	rg, appName := azureDeploymentContext(req.Config, req.Stage, req.Root)
	hostKey, err := azureHostDefaultKey(ctx, req.Config, rg, appName)
	if err != nil {
		return nil, err
	}
	orchestrator := name
	if decls, derr := durableFunctionsFromConfig(req.Config); derr == nil {
		for _, decl := range decls {
			if decl.Name == name {
				orchestrator = decl.Orchestrator
				break
			}
		}
	}
	invokeURL := "https://" + appName + ".azurewebsites.net/runtime/webhooks/durabletask/orchestrators/" + url.PathEscape(orchestrator) + "?code=" + url.QueryEscape(hostKey)
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, invokeURL, bytes.NewReader(req.Payload))
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := apiutil.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("azure durable invoke %s: %s: %s", name, resp.Status, string(payload))
	}
	var out struct {
		ID                 string `json:"id"`
		InstanceID         string `json:"instanceId"`
		StatusQueryGetURI  string `json:"statusQueryGetUri"`
		SendEventPostURI   string `json:"sendEventPostUri"`
		TerminatePostURI   string `json:"terminatePostUri"`
		PurgeHistoryDelete string `json:"purgeHistoryDeleteUri"`
		RewindPostURI      string `json:"rewindPostUri"`
		RestartPostURI     string `json:"restartPostUri"`
		SuspendPostURI     string `json:"suspendPostUri"`
		ResumePostURI      string `json:"resumePostUri"`
	}
	_ = json.Unmarshal(payload, &out)
	runID := strings.TrimSpace(out.InstanceID)
	if runID == "" {
		runID = strings.TrimSpace(out.ID)
	}
	output := "started Durable orchestration"
	if runID != "" {
		output += " (instanceId=" + runID + ")"
	}
	if strings.TrimSpace(out.StatusQueryGetURI) != "" {
		output += " " + out.StatusQueryGetURI
	}
	return &sdkprovider.InvokeResult{
		Provider: "azure-functions",
		Function: "durable:" + name,
		Output:   output,
		RunID:    runID,
		Workflow: name,
	}, nil
}

func (Runner) InspectOrchestrations(ctx context.Context, req sdkprovider.OrchestrationInspectRequest) (map[string]any, error) {
	decls, err := durableFunctionsFromConfig(req.Config)
	if err != nil {
		return nil, err
	}
	if len(decls) == 0 {
		return map[string]any{"durableFunctions": []any{}}, nil
	}
	rg, appName := azureDeploymentContext(req.Config, req.Stage, req.Root)
	hostKey, err := azureHostDefaultKey(ctx, req.Config, rg, appName)
	items := make([]map[string]any, 0, len(decls))
	if err != nil {
		for _, decl := range decls {
			items = append(items, map[string]any{
				"name":         decl.Name,
				"orchestrator": decl.Orchestrator,
				"status":       "credentials-missing",
			})
		}
		return map[string]any{"durableFunctions": items}, nil
	}

	for _, decl := range decls {
		item := map[string]any{
			"name":          decl.Name,
			"orchestrator":  decl.Orchestrator,
			"app":           appName,
			"resourceGroup": rg,
			"console":       azureFunctionAppConsoleLink(req.Config, rg, appName),
		}
		statusURL := "https://" + appName + ".azurewebsites.net/runtime/webhooks/durabletask/instances?code=" + url.QueryEscape(hostKey) + "&top=1"
		httpReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, statusURL, nil)
		resp, reqErr := apiutil.DefaultClient.Do(httpReq)
		if reqErr == nil {
			payload, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				var out struct {
					Instances []map[string]any `json:"instances"`
				}
				_ = json.Unmarshal(payload, &out)
				if len(out.Instances) > 0 {
					item["latestInstance"] = out.Instances[0]
					item["status"] = "active"
				} else {
					item["status"] = "idle"
				}
			}
		}
		if _, ok := item["status"]; !ok {
			item["status"] = "unknown"
		}
		items = append(items, item)
	}
	return map[string]any{"durableFunctions": items}, nil
}

func azureDeploymentContext(cfg sdkprovider.Config, stage, root string) (string, string) {
	coreCfg, _ := sdkbridge.ToCoreConfig(cfg)
	service := "service"
	if coreCfg != nil && strings.TrimSpace(coreCfg.Service) != "" {
		service = coreCfg.Service
	}
	rg := strings.TrimSpace(apiutil.Env("AZURE_RESOURCE_GROUP"))
	appName := ""
	_ = root
	if appName == "" {
		appName = strings.TrimSpace(service + "-" + stage)
	}
	if rg == "" {
		rg = strings.TrimSpace(service + "-" + stage)
	}
	return rg, appName
}

func azureHostDefaultKey(ctx context.Context, cfg sdkprovider.Config, resourceGroup, appName string) (string, error) {
	subID := strings.TrimSpace(apiutil.Env("AZURE_SUBSCRIPTION_ID"))
	token := strings.TrimSpace(apiutil.Env("AZURE_ACCESS_TOKEN"))
	if subID == "" || token == "" {
		return "", fmt.Errorf("AZURE_SUBSCRIPTION_ID and AZURE_ACCESS_TOKEN are required for durable orchestration")
	}
	if strings.TrimSpace(resourceGroup) == "" || strings.TrimSpace(appName) == "" {
		resourceGroup, appName = azureDeploymentContext(cfg, "", "")
	}
	mgmtURL := fmt.Sprintf("%s/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Web/sites/%s/host/default/listKeys?api-version=2022-03-01", strings.TrimRight(azureManagementAPI, "/"), subID, url.PathEscape(resourceGroup), url.PathEscape(appName))
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, mgmtURL, bytes.NewReader([]byte("{}")))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := apiutil.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("azure host keys: %s: %s", resp.Status, string(payload))
	}
	var out struct {
		MasterKey string            `json:"masterKey"`
		Keys      map[string]string `json:"functionKeys"`
	}
	_ = json.Unmarshal(payload, &out)
	if strings.TrimSpace(out.MasterKey) != "" {
		return out.MasterKey, nil
	}
	for _, k := range out.Keys {
		if strings.TrimSpace(k) != "" {
			return k, nil
		}
	}
	return "", fmt.Errorf("no host key returned for function app %s", appName)
}

func azureFunctionAppConsoleLink(cfg sdkprovider.Config, resourceGroup, appName string) string {
	if strings.TrimSpace(resourceGroup) == "" || strings.TrimSpace(appName) == "" {
		return ""
	}
	subID := strings.TrimSpace(apiutil.Env("AZURE_SUBSCRIPTION_ID"))
	if subID == "" {
		return ""
	}
	_ = cfg
	return "https://portal.azure.com/#@/resource/subscriptions/" + subID + "/resourceGroups/" + resourceGroup + "/providers/Microsoft.Web/sites/" + appName + "/overview"
}

func azureSyncDurableAppSettings(ctx context.Context, cfg sdkprovider.Config, resourceGroup, appName string, decls []durableFunctionDecl) (map[string]string, error) {
	settings, err := azureGetAppSettings(ctx, cfg, resourceGroup, appName)
	if err != nil {
		return nil, err
	}
	result := map[string]string{}
	for _, decl := range decls {
		namePrefix := azureDurableSettingPrefix(decl.Name)
		orchestratorKey := namePrefix + "_ORCHESTRATOR"
		managedKey := namePrefix + "_MANAGED"
		taskHubKey := namePrefix + "_TASK_HUB"
		storageKey := namePrefix + "_STORAGE_CONNECTION_SETTING"
		if strings.TrimSpace(settings[managedKey]) == "1" {
			result[decl.Name] = "updated"
		} else {
			result[decl.Name] = "created"
		}
		settings[managedKey] = "1"
		settings[orchestratorKey] = decl.Orchestrator
		if strings.TrimSpace(decl.TaskHub) != "" {
			settings[taskHubKey] = decl.TaskHub
		} else {
			delete(settings, taskHubKey)
		}
		if strings.TrimSpace(decl.StorageConnectionSetting) != "" {
			settings[storageKey] = decl.StorageConnectionSetting
		} else {
			delete(settings, storageKey)
		}
	}
	if err := azurePutAppSettings(ctx, cfg, resourceGroup, appName, settings); err != nil {
		return nil, err
	}
	return result, nil
}

func azureRemoveDurableAppSettings(ctx context.Context, cfg sdkprovider.Config, resourceGroup, appName string, decls []durableFunctionDecl) (map[string]string, error) {
	settings, err := azureGetAppSettings(ctx, cfg, resourceGroup, appName)
	if err != nil {
		return nil, err
	}
	result := map[string]string{}
	for _, decl := range decls {
		namePrefix := azureDurableSettingPrefix(decl.Name)
		managedKey := namePrefix + "_MANAGED"
		orchestratorKey := namePrefix + "_ORCHESTRATOR"
		taskHubKey := namePrefix + "_TASK_HUB"
		storageKey := namePrefix + "_STORAGE_CONNECTION_SETTING"
		if _, ok := settings[managedKey]; ok {
			result[decl.Name] = "deleted"
		} else {
			result[decl.Name] = "absent"
		}
		delete(settings, managedKey)
		delete(settings, orchestratorKey)
		delete(settings, taskHubKey)
		delete(settings, storageKey)
	}
	if err := azurePutAppSettings(ctx, cfg, resourceGroup, appName, settings); err != nil {
		return nil, err
	}
	return result, nil
}

func azureGetAppSettings(ctx context.Context, cfg sdkprovider.Config, resourceGroup, appName string) (map[string]string, error) {
	subID := strings.TrimSpace(apiutil.Env("AZURE_SUBSCRIPTION_ID"))
	token := strings.TrimSpace(apiutil.Env("AZURE_ACCESS_TOKEN"))
	if subID == "" || token == "" {
		return nil, fmt.Errorf("AZURE_SUBSCRIPTION_ID and AZURE_ACCESS_TOKEN are required for durable orchestration")
	}
	if strings.TrimSpace(resourceGroup) == "" || strings.TrimSpace(appName) == "" {
		resourceGroup, appName = azureDeploymentContext(cfg, "", "")
	}
	mgmtURL := fmt.Sprintf("%s/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Web/sites/%s/config/appsettings/list?api-version=2022-03-01", strings.TrimRight(azureManagementAPI, "/"), subID, url.PathEscape(resourceGroup), url.PathEscape(appName))
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, mgmtURL, bytes.NewReader([]byte("{}")))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := apiutil.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("azure app settings list: %s: %s", resp.Status, string(payload))
	}
	var out struct {
		Properties map[string]string `json:"properties"`
	}
	_ = json.Unmarshal(payload, &out)
	if out.Properties == nil {
		out.Properties = map[string]string{}
	}
	return out.Properties, nil
}

func azurePutAppSettings(ctx context.Context, cfg sdkprovider.Config, resourceGroup, appName string, settings map[string]string) error {
	subID := strings.TrimSpace(apiutil.Env("AZURE_SUBSCRIPTION_ID"))
	token := strings.TrimSpace(apiutil.Env("AZURE_ACCESS_TOKEN"))
	if subID == "" || token == "" {
		return fmt.Errorf("AZURE_SUBSCRIPTION_ID and AZURE_ACCESS_TOKEN are required for durable orchestration")
	}
	if strings.TrimSpace(resourceGroup) == "" || strings.TrimSpace(appName) == "" {
		resourceGroup, appName = azureDeploymentContext(cfg, "", "")
	}
	body := map[string]any{"properties": settings}
	bodyBytes, _ := json.Marshal(body)
	mgmtURL := fmt.Sprintf("%s/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Web/sites/%s/config/appsettings?api-version=2022-03-01", strings.TrimRight(azureManagementAPI, "/"), subID, url.PathEscape(resourceGroup), url.PathEscape(appName))
	req, _ := http.NewRequestWithContext(ctx, http.MethodPut, mgmtURL, bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := apiutil.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("azure app settings update: %s: %s", resp.Status, string(payload))
	}
	return nil
}

func azureDurableSettingPrefix(name string) string {
	n := strings.TrimSpace(strings.ToUpper(name))
	if n == "" {
		return "RUNFABRIC_DURABLE_ORCHESTRATION"
	}
	re := regexp.MustCompile(`[^A-Z0-9]+`)
	n = re.ReplaceAllString(n, "_")
	n = strings.Trim(n, "_")
	if n == "" {
		return "RUNFABRIC_DURABLE_ORCHESTRATION"
	}
	return "RUNFABRIC_DURABLE_" + n
}

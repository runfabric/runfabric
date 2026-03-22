package gcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	"github.com/runfabric/runfabric/platform/deploy/apiutil"
)

const gcpWorkflowsAPI = "https://workflowexecutions.googleapis.com/v1"

func (p *Provider) SyncOrchestrations(ctx context.Context, req providers.OrchestrationSyncRequest) (*providers.OrchestrationSyncResult, error) {
	decls, err := cloudWorkflowsFromConfig(req.Config, req.Root)
	if err != nil {
		return nil, err
	}
	if len(decls) == 0 {
		return &providers.OrchestrationSyncResult{}, nil
	}
	project := strings.TrimSpace(apiutil.Env("GCP_PROJECT"))
	if project == "" {
		project = strings.TrimSpace(apiutil.Env("GCP_PROJECT_ID"))
	}
	if project == "" {
		return nil, fmt.Errorf("GCP_PROJECT or GCP_PROJECT_ID is required for cloud workflows")
	}
	region := strings.TrimSpace(req.Config.Provider.Region)
	if region == "" {
		region = "us-central1"
	}
	token := strings.TrimSpace(apiutil.Env("GCP_ACCESS_TOKEN"))
	if token == "" {
		return nil, fmt.Errorf("GCP_ACCESS_TOKEN is required for cloud workflows")
	}

	res := &providers.OrchestrationSyncResult{Metadata: map[string]string{}, Outputs: map[string]string{}}
	for _, decl := range decls {
		source, err := cloudWorkflowDefinitionString(req.Root, decl)
		if err != nil {
			return nil, err
		}
		source = applyCloudWorkflowBindings(source, decl, req.FunctionResourceByName)
		workflowName := fmt.Sprintf("projects/%s/locations/%s/workflows/%s", project, region, decl.Name)
		workflowURL := fmt.Sprintf("%s/projects/%s/locations/%s/workflows/%s", gcpWorkflowsAPI, project, region, decl.Name)

		body := map[string]any{"sourceContents": source}
		bodyBytes, _ := json.Marshal(body)
		patchURL := workflowURL + "?updateMask=sourceContents"
		patchReq, _ := http.NewRequestWithContext(ctx, http.MethodPatch, patchURL, bytes.NewReader(bodyBytes))
		patchReq.Header.Set("Authorization", "Bearer "+token)
		patchReq.Header.Set("Content-Type", "application/json")
		patchResp, err := apiutil.DefaultClient.Do(patchReq)
		if err != nil {
			return nil, err
		}
		patchPayload, _ := io.ReadAll(patchResp.Body)
		patchResp.Body.Close()

		operation := "updated"
		if patchResp.StatusCode == http.StatusNotFound {
			createBody := map[string]any{"workflow": body}
			createBytes, _ := json.Marshal(createBody)
			createURL := fmt.Sprintf("%s/projects/%s/locations/%s/workflows?workflowId=%s", gcpWorkflowsAPI, project, region, url.QueryEscape(decl.Name))
			createReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, createURL, bytes.NewReader(createBytes))
			createReq.Header.Set("Authorization", "Bearer "+token)
			createReq.Header.Set("Content-Type", "application/json")
			createResp, err := apiutil.DefaultClient.Do(createReq)
			if err != nil {
				return nil, err
			}
			createPayload, _ := io.ReadAll(createResp.Body)
			createResp.Body.Close()
			if createResp.StatusCode < 200 || createResp.StatusCode >= 300 {
				return nil, fmt.Errorf("gcp workflow create %s: %s: %s", decl.Name, createResp.Status, string(createPayload))
			}
			operation = "created"
		} else if patchResp.StatusCode < 200 || patchResp.StatusCode >= 300 {
			return nil, fmt.Errorf("gcp workflow patch %s: %s: %s", decl.Name, patchResp.Status, string(patchPayload))
		}

		res.Metadata["cloudworkflow:"+decl.Name+":name"] = workflowName
		res.Metadata["cloudworkflow:"+decl.Name+":operation"] = operation
		res.Metadata["cloudworkflow:"+decl.Name+":console"] = cloudWorkflowConsoleLink(project, region, decl.Name)
	}
	return res, nil
}

func (p *Provider) RemoveOrchestrations(ctx context.Context, req providers.OrchestrationRemoveRequest) (*providers.OrchestrationSyncResult, error) {
	decls, err := cloudWorkflowsFromConfig(req.Config, req.Root)
	if err != nil {
		return nil, err
	}
	if len(decls) == 0 {
		return &providers.OrchestrationSyncResult{}, nil
	}
	project := strings.TrimSpace(apiutil.Env("GCP_PROJECT"))
	if project == "" {
		project = strings.TrimSpace(apiutil.Env("GCP_PROJECT_ID"))
	}
	region := strings.TrimSpace(req.Config.Provider.Region)
	if region == "" {
		region = "us-central1"
	}
	token := strings.TrimSpace(apiutil.Env("GCP_ACCESS_TOKEN"))
	if project == "" || token == "" {
		return &providers.OrchestrationSyncResult{}, nil
	}

	res := &providers.OrchestrationSyncResult{Metadata: map[string]string{}, Outputs: map[string]string{}}
	for _, decl := range decls {
		workflowURL := fmt.Sprintf("%s/projects/%s/locations/%s/workflows/%s", gcpWorkflowsAPI, project, region, decl.Name)
		delReq, _ := http.NewRequestWithContext(ctx, http.MethodDelete, workflowURL, nil)
		delReq.Header.Set("Authorization", "Bearer "+token)
		delResp, err := apiutil.DefaultClient.Do(delReq)
		if err != nil {
			return nil, err
		}
		payload, _ := io.ReadAll(delResp.Body)
		delResp.Body.Close()
		if delResp.StatusCode == http.StatusNotFound {
			res.Metadata["cloudworkflow:"+decl.Name+":operation"] = "absent"
			continue
		}
		if delResp.StatusCode < 200 || delResp.StatusCode >= 300 {
			return nil, fmt.Errorf("gcp workflow delete %s: %s: %s", decl.Name, delResp.Status, string(payload))
		}
		res.Metadata["cloudworkflow:"+decl.Name+":name"] = fmt.Sprintf("projects/%s/locations/%s/workflows/%s", project, region, decl.Name)
		res.Metadata["cloudworkflow:"+decl.Name+":operation"] = "deleted"
	}
	return res, nil
}

func (p *Provider) InvokeOrchestration(ctx context.Context, req providers.OrchestrationInvokeRequest) (*providers.InvokeResult, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("orchestration name is required")
	}
	project := strings.TrimSpace(apiutil.Env("GCP_PROJECT"))
	if project == "" {
		project = strings.TrimSpace(apiutil.Env("GCP_PROJECT_ID"))
	}
	if project == "" {
		return nil, fmt.Errorf("GCP_PROJECT or GCP_PROJECT_ID is required for cloud workflows")
	}
	region := strings.TrimSpace(req.Config.Provider.Region)
	if region == "" {
		region = "us-central1"
	}
	token := strings.TrimSpace(apiutil.Env("GCP_ACCESS_TOKEN"))
	if token == "" {
		return nil, fmt.Errorf("GCP_ACCESS_TOKEN is required for cloud workflows")
	}

	argument := strings.TrimSpace(string(req.Payload))
	if argument == "" {
		argument = "{}"
	}
	body := map[string]any{"argument": argument}
	bodyBytes, _ := json.Marshal(body)
	execURL := fmt.Sprintf("%s/projects/%s/locations/%s/workflows/%s/executions", gcpWorkflowsAPI, project, region, name)
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, execURL, bytes.NewReader(bodyBytes))
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := apiutil.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gcp workflow invoke %s: %s: %s", name, resp.Status, string(payload))
	}
	var out struct {
		Name  string `json:"name"`
		State string `json:"state"`
	}
	_ = json.Unmarshal(payload, &out)
	output := "started Cloud Workflow execution"
	if strings.TrimSpace(out.State) != "" {
		output += " (state=" + out.State + ")"
	}
	if out.Name != "" {
		output += " " + cloudWorkflowExecutionConsoleLink(project, region, name, out.Name)
	}
	return &providers.InvokeResult{
		Provider: p.Name(),
		Function: "cwf:" + name,
		Output:   output,
		RunID:    out.Name,
		Workflow: name,
	}, nil
}

func (p *Provider) InspectOrchestrations(ctx context.Context, req providers.OrchestrationInspectRequest) (map[string]any, error) {
	decls, err := cloudWorkflowsFromConfig(req.Config, req.Root)
	if err != nil {
		return nil, err
	}
	if len(decls) == 0 {
		return map[string]any{"cloudWorkflows": []any{}}, nil
	}
	project := strings.TrimSpace(apiutil.Env("GCP_PROJECT"))
	if project == "" {
		project = strings.TrimSpace(apiutil.Env("GCP_PROJECT_ID"))
	}
	region := strings.TrimSpace(req.Config.Provider.Region)
	if region == "" {
		region = "us-central1"
	}
	token := strings.TrimSpace(apiutil.Env("GCP_ACCESS_TOKEN"))
	items := make([]map[string]any, 0, len(decls))
	if project == "" || token == "" {
		for _, decl := range decls {
			items = append(items, map[string]any{"name": decl.Name, "declared": true, "status": "credentials-missing"})
		}
		return map[string]any{"cloudWorkflows": items}, nil
	}

	for _, decl := range decls {
		item := map[string]any{"name": decl.Name, "declared": true}
		workflowURL := fmt.Sprintf("%s/projects/%s/locations/%s/workflows/%s", gcpWorkflowsAPI, project, region, decl.Name)
		getReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, workflowURL, nil)
		getReq.Header.Set("Authorization", "Bearer "+token)
		resp, err := apiutil.DefaultClient.Do(getReq)
		if err != nil {
			return nil, err
		}
		payload, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			item["status"] = "absent"
			items = append(items, item)
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("gcp workflow inspect %s: %s: %s", decl.Name, resp.Status, string(payload))
		}
		var wf struct {
			Name  string `json:"name"`
			State string `json:"state"`
		}
		_ = json.Unmarshal(payload, &wf)
		if wf.Name != "" {
			item["resource"] = wf.Name
		}
		if wf.State != "" {
			item["status"] = wf.State
		}
		item["console"] = cloudWorkflowConsoleLink(project, region, decl.Name)

		execURL := fmt.Sprintf("%s/projects/%s/locations/%s/workflows/%s/executions?pageSize=1", gcpWorkflowsAPI, project, region, decl.Name)
		execReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, execURL, nil)
		execReq.Header.Set("Authorization", "Bearer "+token)
		execResp, err := apiutil.DefaultClient.Do(execReq)
		if err == nil {
			execPayload, _ := io.ReadAll(execResp.Body)
			execResp.Body.Close()
			if execResp.StatusCode >= 200 && execResp.StatusCode < 300 {
				var executions struct {
					Executions []struct {
						Name     string `json:"name"`
						State    string `json:"state"`
						Start    string `json:"startTime"`
						End      string `json:"endTime"`
						Revision string `json:"workflowRevisionId"`
					} `json:"executions"`
				}
				_ = json.Unmarshal(execPayload, &executions)
				if len(executions.Executions) > 0 {
					latest := executions.Executions[0]
					item["latestExecution"] = map[string]any{
						"name":               latest.Name,
						"state":              latest.State,
						"startTime":          latest.Start,
						"endTime":            latest.End,
						"workflowRevisionId": latest.Revision,
						"console":            cloudWorkflowExecutionConsoleLink(project, region, decl.Name, latest.Name),
					}
				}
			}
		}
		items = append(items, item)
	}
	return map[string]any{"cloudWorkflows": items}, nil
}

func cloudWorkflowConsoleLink(project, region, name string) string {
	if strings.TrimSpace(project) == "" || strings.TrimSpace(region) == "" || strings.TrimSpace(name) == "" {
		return ""
	}
	return "https://console.cloud.google.com/workflows/workflow/" + region + "/" + name + "/details?project=" + project
}

func cloudWorkflowExecutionConsoleLink(project, region, workflowName, executionName string) string {
	if strings.TrimSpace(project) == "" || strings.TrimSpace(region) == "" || strings.TrimSpace(workflowName) == "" || strings.TrimSpace(executionName) == "" {
		return ""
	}
	shortExec := executionName
	if idx := strings.LastIndex(executionName, "/"); idx >= 0 && idx+1 < len(executionName) {
		shortExec = executionName[idx+1:]
	}
	return "https://console.cloud.google.com/workflows/workflow/" + region + "/" + workflowName + "/execution/" + shortExec + "?project=" + project
}

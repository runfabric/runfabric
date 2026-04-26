package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	state "github.com/runfabric/runfabric/platform/core/state/core"
)

// StepTelemetryHook records AI step execution metrics after each step completes.
// Implementations send metrics to provider-native monitoring systems.
type StepTelemetryHook interface {
	RecordStep(run *state.WorkflowRun, step state.WorkflowStepRun, result *StepExecutionResult, elapsed time.Duration, err error)
}

// NoopTelemetryHook discards all telemetry events. Used as the default.
type NoopTelemetryHook struct{}

func (NoopTelemetryHook) RecordStep(_ *state.WorkflowRun, _ state.WorkflowStepRun, _ *StepExecutionResult, _ time.Duration, _ error) {
}

// telemetryClient is a shared HTTP client for all telemetry hooks.
// Short timeout: telemetry must not block workflow execution.
var telemetryClient = &http.Client{Timeout: 5 * time.Second}

// AWSCloudWatchHook records step metrics via CloudWatch PutMetricData.
type AWSCloudWatchHook struct {
	Namespace string // defaults to "RunFabric/Workflow" if empty
	Region    string
}

func (h AWSCloudWatchHook) RecordStep(run *state.WorkflowRun, step state.WorkflowStepRun, _ *StepExecutionResult, elapsed time.Duration, err error) {
	region := h.Region
	if region == "" {
		region = os.Getenv("AWS_REGION")
	}
	if region == "" {
		return
	}
	ns := h.Namespace
	if ns == "" {
		ns = "RunFabric/Workflow"
	}
	status := "Success"
	if err != nil {
		status = "Error"
	}

	form := url.Values{}
	form.Set("Action", "PutMetricData")
	form.Set("Version", "2010-08-01")
	form.Set("Namespace", ns)
	form.Set("MetricData.member.1.MetricName", "StepDuration")
	form.Set("MetricData.member.1.Unit", "Milliseconds")
	form.Set("MetricData.member.1.Value", fmt.Sprintf("%d", elapsed.Milliseconds()))
	form.Set("MetricData.member.1.Dimensions.member.1.Name", "WorkflowHash")
	form.Set("MetricData.member.1.Dimensions.member.1.Value", run.WorkflowHash)
	form.Set("MetricData.member.1.Dimensions.member.2.Name", "StepKind")
	form.Set("MetricData.member.1.Dimensions.member.2.Value", step.Kind)
	form.Set("MetricData.member.1.Dimensions.member.3.Name", "Status")
	form.Set("MetricData.member.1.Dimensions.member.3.Value", status)
	form.Set("MetricData.member.1.Dimensions.member.4.Name", "Region")
	form.Set("MetricData.member.1.Dimensions.member.4.Value", region)
	body := []byte(form.Encode())

	cwURL := fmt.Sprintf("https://monitoring.%s.amazonaws.com/", region)
	req, err2 := http.NewRequestWithContext(context.Background(), http.MethodPost, cwURL, bytes.NewReader(body))
	if err2 != nil {
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if signErr := signAWSRequest(req, body, region, "monitoring"); signErr != nil {
		return
	}
	resp, err2 := telemetryClient.Do(req)
	if err2 != nil {
		return
	}
	io.Copy(io.Discard, resp.Body) //nolint:errcheck
	resp.Body.Close()
}

// GCPCloudMonitoringHook records step metrics via Cloud Monitoring timeSeries.create.
type GCPCloudMonitoringHook struct {
	Project string // GCP project ID; falls back to RUNFABRIC_GCP_PROJECT_ID / GOOGLE_CLOUD_PROJECT
	Region  string
}

func (h GCPCloudMonitoringHook) RecordStep(run *state.WorkflowRun, step state.WorkflowStepRun, _ *StepExecutionResult, elapsed time.Duration, err error) {
	project := h.Project
	if project == "" {
		project = os.Getenv("RUNFABRIC_GCP_PROJECT_ID")
	}
	if project == "" {
		project = os.Getenv("GOOGLE_CLOUD_PROJECT")
	}
	token := strings.TrimSpace(os.Getenv("RUNFABRIC_GCP_ACCESS_TOKEN"))
	if token == "" {
		token = strings.TrimSpace(os.Getenv("GOOGLE_ACCESS_TOKEN"))
	}
	if project == "" || token == "" {
		return
	}

	status := "OK"
	if err != nil {
		status = "ERROR"
	}

	payload := map[string]any{
		"timeSeries": []map[string]any{{
			"metric": map[string]any{
				"type": "custom.googleapis.com/runfabric/step_duration",
				"labels": map[string]string{
					"workflow_hash": run.WorkflowHash,
					"step_kind":     step.Kind,
					"status":        status,
				},
			},
			"resource": map[string]any{
				"type":   "global",
				"labels": map[string]string{"project_id": project},
			},
			"points": []map[string]any{{
				"interval": map[string]string{"endTime": time.Now().UTC().Format(time.RFC3339)},
				"value":    map[string]any{"doubleValue": elapsed.Seconds()},
			}},
		}},
	}
	body, err2 := json.Marshal(payload)
	if err2 != nil {
		return
	}

	gcpURL := fmt.Sprintf("https://monitoring.googleapis.com/v3/projects/%s/timeSeries", project)
	req, err2 := http.NewRequestWithContext(context.Background(), http.MethodPost, gcpURL, bytes.NewReader(body))
	if err2 != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err2 := telemetryClient.Do(req)
	if err2 != nil {
		return
	}
	io.Copy(io.Discard, resp.Body) //nolint:errcheck
	resp.Body.Close()
}

// AzureMonitorHook records step metrics via Azure Monitor custom metrics ingestion.
// Requires AZURE_MONITOR_ACCESS_TOKEN (Azure AD bearer token).
// Subscription and ResourceGroup fall back to AZURE_SUBSCRIPTION_ID / AZURE_RESOURCE_GROUP
// if not set on the struct. AppName falls back to AZURE_FUNCTION_APP_NAME.
type AzureMonitorHook struct {
	Subscription  string
	ResourceGroup string
	Region        string
	AppName       string
}

func (h AzureMonitorHook) RecordStep(run *state.WorkflowRun, step state.WorkflowStepRun, _ *StepExecutionResult, elapsed time.Duration, err error) {
	sub := h.Subscription
	if sub == "" {
		sub = os.Getenv("AZURE_SUBSCRIPTION_ID")
	}
	rg := h.ResourceGroup
	if rg == "" {
		rg = os.Getenv("AZURE_RESOURCE_GROUP")
	}
	appName := h.AppName
	if appName == "" {
		appName = os.Getenv("AZURE_FUNCTION_APP_NAME")
	}
	token := os.Getenv("AZURE_MONITOR_ACCESS_TOKEN")
	if sub == "" || rg == "" || appName == "" || token == "" {
		return
	}

	status := "Succeeded"
	if err != nil {
		status = "Failed"
	}

	// Azure Monitor custom metrics:
	// https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/metrics-store-custom-rest-api
	azURL := fmt.Sprintf(
		"https://%s.monitoring.azure.com/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Web/sites/%s/metrics",
		h.Region, sub, rg, appName,
	)

	payload := map[string]any{
		"time": time.Now().UTC().Format(time.RFC3339),
		"data": map[string]any{
			"baseData": map[string]any{
				"metric":    "StepDuration",
				"namespace": "RunFabric",
				"dimNames":  []string{"WorkflowHash", "StepKind", "Status"},
				"series": []map[string]any{{
					"dimValues": []string{run.WorkflowHash, step.Kind, status},
					"sum":       float64(elapsed.Milliseconds()),
					"count":     1,
				}},
			},
		},
	}
	body, err2 := json.Marshal(payload)
	if err2 != nil {
		return
	}

	req, err2 := http.NewRequestWithContext(context.Background(), http.MethodPost, azURL, bytes.NewReader(body))
	if err2 != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err2 := telemetryClient.Do(req)
	if err2 != nil {
		return
	}
	io.Copy(io.Discard, resp.Body) //nolint:errcheck
	resp.Body.Close()
}

// ProviderTelemetryHook returns a provider-appropriate StepTelemetryHook.
// Falls back to NoopTelemetryHook for unknown or empty providers.
func ProviderTelemetryHook(provider, region, project string) StepTelemetryHook {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "aws-lambda":
		return AWSCloudWatchHook{Region: region}
	case "gcp-functions":
		return GCPCloudMonitoringHook{Project: project, Region: region}
	case "azure-functions":
		return AzureMonitorHook{Region: region}
	default:
		return NoopTelemetryHook{}
	}
}

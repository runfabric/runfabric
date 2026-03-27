package controlplane

import (
	"fmt"
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

// AWSCloudWatchHook records step metrics in AWS CloudWatch custom metrics format.
// In production this would call cloudwatch.PutMetricData; here it constructs the envelope.
type AWSCloudWatchHook struct {
	Namespace string // e.g. "RunFabric/Workflow"; defaults to "RunFabric/Workflow" if empty.
	Region    string
}

func (h AWSCloudWatchHook) RecordStep(run *state.WorkflowRun, step state.WorkflowStepRun, _ *StepExecutionResult, elapsed time.Duration, err error) {
	ns := h.Namespace
	if ns == "" {
		ns = "RunFabric/Workflow"
	}
	status := "Success"
	if err != nil {
		status = "Error"
	}
	// Metric envelope — production callers replace this with a real PutMetricData call.
	_ = map[string]any{
		"Namespace": ns,
		"MetricData": []map[string]any{{
			"MetricName": "StepDuration",
			"Dimensions": []map[string]string{
				{"Name": "WorkflowHash", "Value": run.WorkflowHash},
				{"Name": "StepKind", "Value": step.Kind},
				{"Name": "Status", "Value": status},
				{"Name": "Region", "Value": h.Region},
			},
			"Value": elapsed.Milliseconds(),
			"Unit":  "Milliseconds",
		}},
	}
}

// GCPCloudMonitoringHook records step metrics in GCP Cloud Monitoring time-series format.
// In production this would call monitoring.ProjectsTimeSeriesCreate.
type GCPCloudMonitoringHook struct {
	Project string // GCP project ID; defaults to "runfabric-project" if empty.
	Region  string
}

func (h GCPCloudMonitoringHook) RecordStep(run *state.WorkflowRun, step state.WorkflowStepRun, _ *StepExecutionResult, elapsed time.Duration, err error) {
	project := h.Project
	if project == "" {
		project = "runfabric-project"
	}
	status := "OK"
	if err != nil {
		status = "ERROR"
	}
	_ = map[string]any{
		"name":   fmt.Sprintf("projects/%s/timeSeries", project),
		"metric": "custom.googleapis.com/runfabric/step_duration",
		"resource": map[string]string{
			"type":    "global",
			"project": project,
			"region":  h.Region,
		},
		"points": []map[string]any{{
			"interval": map[string]string{"endTime": time.Now().UTC().Format(time.RFC3339)},
			"value":    map[string]any{"doubleValue": elapsed.Seconds()},
		}},
		"labels": map[string]string{
			"workflow_hash": run.WorkflowHash,
			"step_kind":     step.Kind,
			"status":        status,
		},
	}
}

// AzureMonitorHook records step metrics in Azure Monitor custom metrics format.
// In production this would POST to the Azure Monitor ingestion endpoint.
type AzureMonitorHook struct {
	Subscription  string
	ResourceGroup string
	Region        string
}

func (h AzureMonitorHook) RecordStep(run *state.WorkflowRun, step state.WorkflowStepRun, _ *StepExecutionResult, elapsed time.Duration, err error) {
	status := "Succeeded"
	if err != nil {
		status = "Failed"
	}
	_ = map[string]any{
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
}

// ProviderTelemetryHook returns a provider-appropriate StepTelemetryHook.
// Falls back to NoopTelemetryHook for unknown or empty providers.
func ProviderTelemetryHook(provider, region, project string) StepTelemetryHook {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "aws":
		return AWSCloudWatchHook{Region: region}
	case "gcp":
		return GCPCloudMonitoringHook{Project: project, Region: region}
	case "azure":
		return AzureMonitorHook{Region: region}
	default:
		return NoopTelemetryHook{}
	}
}

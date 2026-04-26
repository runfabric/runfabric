package contracts

import (
	"context"

	planner "github.com/runfabric/runfabric/platform/planner/engine"
)

// DoctorResult is the result of Provider.Doctor.
type DoctorResult struct {
	Provider string   `json:"provider"`
	Checks   []string `json:"checks"`
}

// PlanResult is the result of Provider.Plan.
type PlanResult struct {
	Provider string        `json:"provider"`
	Plan     *planner.Plan `json:"plan"`
	Warnings []string      `json:"warnings,omitempty"`
}

// Artifact describes a built artifact for a function.
type Artifact struct {
	Function        string `json:"function"`
	Runtime         string `json:"runtime"`
	SourcePath      string `json:"sourcePath"`
	OutputPath      string `json:"outputPath"`
	SHA256          string `json:"sha256"`
	SizeBytes       int64  `json:"sizeBytes"`
	ConfigSignature string `json:"configSignature,omitempty"`
}

// DeployedFunction describes provider-resolved deployment details for a logical function.
type DeployedFunction struct {
	ResourceName       string            `json:"resourceName,omitempty"`
	ResourceIdentifier string            `json:"resourceIdentifier,omitempty"`
	Metadata           map[string]string `json:"metadata,omitempty"`
}

// DeployResult is the result of Provider.Deploy.
type DeployResult struct {
	Provider     string                      `json:"provider"`
	DeploymentID string                      `json:"deploymentId"`
	Outputs      map[string]string           `json:"outputs"`
	Artifacts    []Artifact                  `json:"artifacts,omitempty"`
	Metadata     map[string]string           `json:"metadata,omitempty"`
	Functions    map[string]DeployedFunction `json:"functions,omitempty"`
}

// RemoveResult is the result of Provider.Remove.
type RemoveResult struct {
	Provider string `json:"provider"`
	Removed  bool   `json:"removed"`
}

// InvokeResult is the result of Provider.Invoke.
type InvokeResult struct {
	Provider string `json:"provider"`
	Function string `json:"function"`
	Output   string `json:"output"`
	RunID    string `json:"runId,omitempty"`
	Workflow string `json:"workflowHash,omitempty"`
}

// LogsResult is the result of Provider.Logs.
type LogsResult struct {
	Provider string   `json:"provider"`
	Function string   `json:"function"`
	Lines    []string `json:"lines"`
	Workflow string   `json:"workflowHash,omitempty"`
}

// OrchestrationSyncResult contains metadata/outputs from orchestration sync operations.
type OrchestrationSyncResult struct {
	Metadata map[string]string `json:"metadata,omitempty"`
	Outputs  map[string]string `json:"outputs,omitempty"`
}

// ProviderPlugin is the primary interface for provider plugins.
type ProviderPlugin interface {
	Meta() ProviderMeta
	ValidateConfig(ctx context.Context, req ValidateConfigRequest) error
	Doctor(ctx context.Context, req DoctorRequest) (*DoctorResult, error)
	Plan(ctx context.Context, req PlanRequest) (*PlanResult, error)
	Deploy(ctx context.Context, req DeployRequest) (*DeployResult, error)
	Remove(ctx context.Context, req RemoveRequest) (*RemoveResult, error)
	Invoke(ctx context.Context, req InvokeRequest) (*InvokeResult, error)
	Logs(ctx context.Context, req LogsRequest) (*LogsResult, error)
}

// ProviderRegistry is the single source of truth for provider resolution.
type ProviderRegistry interface {
	Register(p ProviderPlugin) error
	Get(name string) (ProviderPlugin, bool)
	List() []ProviderMeta
}

// OrchestrationCapable is an optional capability for providers that manage orchestration resources.
type OrchestrationCapable interface {
	SyncOrchestrations(ctx context.Context, req OrchestrationSyncRequest) (*OrchestrationSyncResult, error)
	RemoveOrchestrations(ctx context.Context, req OrchestrationRemoveRequest) (*OrchestrationSyncResult, error)
	InvokeOrchestration(ctx context.Context, req OrchestrationInvokeRequest) (*InvokeResult, error)
	InspectOrchestrations(ctx context.Context, req OrchestrationInspectRequest) (map[string]any, error)
}

// ObservabilityCapable is an optional capability for provider-native metrics and traces.
type ObservabilityCapable interface {
	FetchMetrics(ctx context.Context, req MetricsRequest) (*MetricsResult, error)
	FetchTraces(ctx context.Context, req TracesRequest) (*TracesResult, error)
}

// DevStreamCapable is an optional capability for provider-side tunnel redirect/restore.
type DevStreamCapable interface {
	PrepareDevStream(ctx context.Context, req DevStreamRequest) (*DevStreamSession, error)
}

// RecoveryCapable is an optional capability for provider-specific recovery flows.
type RecoveryCapable interface {
	Recover(ctx context.Context, req RecoveryRequest) (*RecoveryResult, error)
}

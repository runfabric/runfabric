package provider

import "context"

// Config carries provider configuration payload from runfabric.yml.
// The SDK keeps this schema-free to avoid coupling plugin repos to engine internals.
type Config map[string]any

type Meta struct {
	Name              string   `json:"name"`
	Version           string   `json:"version,omitempty"`
	PluginVersion     string   `json:"pluginVersion,omitempty"`
	Capabilities      []string `json:"capabilities,omitempty"`
	SupportsRuntime   []string `json:"supportsRuntime,omitempty"`
	SupportsTriggers  []string `json:"supportsTriggers,omitempty"`
	SupportsResources []string `json:"supportsResources,omitempty"`
}

type ValidateConfigRequest struct {
	Config Config `json:"config,omitempty"`
	Stage  string `json:"stage,omitempty"`
}

type DoctorRequest struct {
	Config Config `json:"config,omitempty"`
	Stage  string `json:"stage,omitempty"`
}

type DoctorResult struct {
	Provider string   `json:"provider"`
	Checks   []string `json:"checks"`
}

type PlanRequest struct {
	Config Config `json:"config,omitempty"`
	Stage  string `json:"stage,omitempty"`
	Root   string `json:"root,omitempty"`
}

type PlanResult struct {
	Provider string   `json:"provider"`
	Plan     any      `json:"plan,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

type Artifact struct {
	Function        string `json:"function"`
	Runtime         string `json:"runtime"`
	SourcePath      string `json:"sourcePath,omitempty"`
	OutputPath      string `json:"outputPath,omitempty"`
	SHA256          string `json:"sha256,omitempty"`
	SizeBytes       int64  `json:"sizeBytes,omitempty"`
	ConfigSignature string `json:"configSignature,omitempty"`
}

type DeployedFunction struct {
	ResourceName       string            `json:"resourceName,omitempty"`
	ResourceIdentifier string            `json:"resourceIdentifier,omitempty"`
	Metadata           map[string]string `json:"metadata,omitempty"`
}

type DeployRequest struct {
	Config Config `json:"config,omitempty"`
	Stage  string `json:"stage,omitempty"`
	Root   string `json:"root,omitempty"`
}

type DeployResult struct {
	Provider     string                      `json:"provider"`
	DeploymentID string                      `json:"deploymentId"`
	Outputs      map[string]string           `json:"outputs,omitempty"`
	Artifacts    []Artifact                  `json:"artifacts,omitempty"`
	Metadata     map[string]string           `json:"metadata,omitempty"`
	Functions    map[string]DeployedFunction `json:"functions,omitempty"`
}

type RemoveRequest struct {
	Config  Config `json:"config,omitempty"`
	Stage   string `json:"stage,omitempty"`
	Root    string `json:"root,omitempty"`
	Receipt any    `json:"receipt,omitempty"`
}

type RemoveResult struct {
	Provider string `json:"provider"`
	Removed  bool   `json:"removed"`
}

type InvokeRequest struct {
	Config   Config `json:"config,omitempty"`
	Stage    string `json:"stage,omitempty"`
	Function string `json:"function,omitempty"`
	Payload  []byte `json:"payload,omitempty"`
}

type InvokeResult struct {
	Provider string `json:"provider"`
	Function string `json:"function,omitempty"`
	Output   string `json:"output,omitempty"`
	RunID    string `json:"runId,omitempty"`
	Workflow string `json:"workflowHash,omitempty"`
}

type LogsRequest struct {
	Config   Config `json:"config,omitempty"`
	Stage    string `json:"stage,omitempty"`
	Function string `json:"function,omitempty"`
}

type LogsResult struct {
	Provider string   `json:"provider"`
	Function string   `json:"function,omitempty"`
	Lines    []string `json:"lines,omitempty"`
	Workflow string   `json:"workflowHash,omitempty"`
}

type MetricsRequest struct {
	Config Config `json:"config,omitempty"`
	Stage  string `json:"stage,omitempty"`
}

type MetricsResult struct {
	PerFunction map[string]any `json:"perFunction,omitempty"`
	Message     string         `json:"message,omitempty"`
}

type TracesRequest struct {
	Config Config `json:"config,omitempty"`
	Stage  string `json:"stage,omitempty"`
}

type TracesResult struct {
	Traces  []any  `json:"traces,omitempty"`
	Message string `json:"message,omitempty"`
}

type DevStreamRequest struct {
	Config    Config `json:"config,omitempty"`
	Stage     string `json:"stage,omitempty"`
	TunnelURL string `json:"tunnelURL,omitempty"`
	Region    string `json:"region,omitempty"`
}

// DevStreamSession captures provider-side tunnel redirect state for API workflows.
// RestoreData can carry provider-specific state that callers persist externally.
// restore is an unexported, non-serialized callback used by in-process plugins
// to revert provider-side state when the dev-stream session exits.
type DevStreamSession struct {
	EffectiveMode  string         `json:"effectiveMode,omitempty"`
	MissingPrereqs []string       `json:"missingPrereqs,omitempty"`
	StatusMessage  string         `json:"statusMessage,omitempty"`
	RestoreData    map[string]any `json:"restoreData,omitempty"`
	restore        func(context.Context) error
}

// NewDevStreamSession builds a session with an optional in-process restore callback.
// The callback is never serialized; external plugins that communicate over the
// wire protocol should leave restore as nil and use RestoreData instead.
func NewDevStreamSession(mode string, missing []string, message string, restore func(context.Context) error) *DevStreamSession {
	return &DevStreamSession{
		EffectiveMode:  mode,
		MissingPrereqs: append([]string(nil), missing...),
		StatusMessage:  message,
		restore:        restore,
	}
}

// Restore calls the in-process restore callback if one was registered.
// It is a no-op when the session is nil or the callback is absent.
func (s *DevStreamSession) Restore(ctx context.Context) error {
	if s == nil || s.restore == nil {
		return nil
	}
	return s.restore(ctx)
}

type RecoveryRequest struct {
	Config  Config `json:"config,omitempty"`
	Root    string `json:"root,omitempty"`
	Service string `json:"service,omitempty"`
	Stage   string `json:"stage,omitempty"`
	Region  string `json:"region,omitempty"`
	Mode    string `json:"mode,omitempty"`
	Journal any    `json:"journal,omitempty"`
}

type RecoveryResult struct {
	Recovered  bool              `json:"recovered"`
	Mode       string            `json:"mode"`
	Status     string            `json:"status"`
	Message    string            `json:"message,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	Errors     []string          `json:"errors,omitempty"`
	ResumeData map[string]any    `json:"resumeData,omitempty"`
}

type OrchestrationSyncRequest struct {
	Config                 Config            `json:"config,omitempty"`
	Stage                  string            `json:"stage,omitempty"`
	Root                   string            `json:"root,omitempty"`
	FunctionResourceByName map[string]string `json:"functionResourceByName,omitempty"`
}

type OrchestrationRemoveRequest struct {
	Config Config `json:"config,omitempty"`
	Stage  string `json:"stage,omitempty"`
	Root   string `json:"root,omitempty"`
}

type OrchestrationInvokeRequest struct {
	Config  Config `json:"config,omitempty"`
	Stage   string `json:"stage,omitempty"`
	Root    string `json:"root,omitempty"`
	Name    string `json:"name,omitempty"`
	Payload []byte `json:"payload,omitempty"`
}

type OrchestrationInspectRequest struct {
	Config Config `json:"config,omitempty"`
	Stage  string `json:"stage,omitempty"`
	Root   string `json:"root,omitempty"`
}

type OrchestrationSyncResult struct {
	Metadata map[string]string `json:"metadata,omitempty"`
	Outputs  map[string]string `json:"outputs,omitempty"`
}

type Plugin interface {
	Meta() Meta
	ValidateConfig(ctx context.Context, req ValidateConfigRequest) error
	Doctor(ctx context.Context, req DoctorRequest) (*DoctorResult, error)
	Plan(ctx context.Context, req PlanRequest) (*PlanResult, error)
	Deploy(ctx context.Context, req DeployRequest) (*DeployResult, error)
	Remove(ctx context.Context, req RemoveRequest) (*RemoveResult, error)
	Invoke(ctx context.Context, req InvokeRequest) (*InvokeResult, error)
	Logs(ctx context.Context, req LogsRequest) (*LogsResult, error)
}

type ObservabilityCapable interface {
	FetchMetrics(ctx context.Context, req MetricsRequest) (*MetricsResult, error)
	FetchTraces(ctx context.Context, req TracesRequest) (*TracesResult, error)
}

type DevStreamCapable interface {
	PrepareDevStream(ctx context.Context, req DevStreamRequest) (*DevStreamSession, error)
}

type RecoveryCapable interface {
	Recover(ctx context.Context, req RecoveryRequest) (*RecoveryResult, error)
}

type OrchestrationCapable interface {
	SyncOrchestrations(ctx context.Context, req OrchestrationSyncRequest) (*OrchestrationSyncResult, error)
	RemoveOrchestrations(ctx context.Context, req OrchestrationRemoveRequest) (*OrchestrationSyncResult, error)
	InvokeOrchestration(ctx context.Context, req OrchestrationInvokeRequest) (*InvokeResult, error)
	InspectOrchestrations(ctx context.Context, req OrchestrationInspectRequest) (map[string]any, error)
}

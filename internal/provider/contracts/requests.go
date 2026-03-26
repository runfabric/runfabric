package contracts

import "context"

// ProviderMeta is the plugin metadata returned by ProviderPlugin.Meta().
type ProviderMeta struct {
	Name              string
	Version           string
	PluginVersion     string
	Capabilities      []string
	SupportsRuntime   []string
	SupportsTriggers  []string
	SupportsResources []string
}

// ValidateConfigRequest is the input to ValidateConfig.
type ValidateConfigRequest struct {
	Config *Config
	Stage  string
}

// DoctorRequest is the input to Doctor.
type DoctorRequest struct {
	Config *Config
	Stage  string
}

// PlanRequest is the input to Plan.
type PlanRequest struct {
	Config *Config
	Stage  string
	Root   string
}

// DeployRequest is the input to Deploy.
type DeployRequest struct {
	Config *Config
	Stage  string
	Root   string
}

// RemoveRequest is the input to Remove.
type RemoveRequest struct {
	Config  *Config
	Stage   string
	Root    string
	Receipt interface{}
}

// InvokeRequest is the input to Invoke.
type InvokeRequest struct {
	Config   *Config
	Stage    string
	Function string
	Payload  []byte
}

// LogsRequest is the input to Logs.
type LogsRequest struct {
	Config   *Config
	Stage    string
	Function string
}

// OrchestrationSyncRequest is the input to SyncOrchestrations.
type OrchestrationSyncRequest struct {
	Config                 *Config
	Stage                  string
	Root                   string
	FunctionResourceByName map[string]string
}

// OrchestrationRemoveRequest is the input to RemoveOrchestrations.
type OrchestrationRemoveRequest struct {
	Config *Config
	Stage  string
	Root   string
}

// OrchestrationInvokeRequest is the input to InvokeOrchestration.
type OrchestrationInvokeRequest struct {
	Config  *Config
	Stage   string
	Root    string
	Name    string
	Payload []byte
}

// OrchestrationInspectRequest is the input to InspectOrchestrations.
type OrchestrationInspectRequest struct {
	Config *Config
	Stage  string
	Root   string
}

// MetricsRequest is the input to FetchMetrics.
type MetricsRequest struct {
	Config *Config
	Stage  string
}

// MetricsResult contains provider metrics payload.
type MetricsResult struct {
	PerFunction map[string]any `json:"perFunction,omitempty"`
	Message     string         `json:"message,omitempty"`
}

// TracesRequest is the input to FetchTraces.
type TracesRequest struct {
	Config *Config
	Stage  string
}

// TracesResult contains provider trace payload.
type TracesResult struct {
	Traces  []any  `json:"traces,omitempty"`
	Message string `json:"message,omitempty"`
}

// DevStreamRequest is the input to PrepareDevStream.
type DevStreamRequest struct {
	Config    *Config
	Stage     string
	TunnelURL string
	Region    string
}

// DevStreamSession captures a reversible provider-side tunnel redirect.
type DevStreamSession struct {
	EffectiveMode  string   `json:"effectiveMode,omitempty"`
	MissingPrereqs []string `json:"missingPrereqs,omitempty"`
	StatusMessage  string   `json:"statusMessage,omitempty"`
	restore        func(context.Context) error
}

// NewDevStreamSession builds a session with an optional restore callback.
func NewDevStreamSession(mode string, missing []string, message string, restore func(context.Context) error) *DevStreamSession {
	return &DevStreamSession{
		EffectiveMode:  mode,
		MissingPrereqs: append([]string(nil), missing...),
		StatusMessage:  message,
		restore:        restore,
	}
}

// Restore reverts provider-side tunnel redirect when supported.
func (s *DevStreamSession) Restore(ctx context.Context) error {
	if s == nil || s.restore == nil {
		return nil
	}
	return s.restore(ctx)
}

// RecoveryRequest is the input to Recover.
type RecoveryRequest struct {
	Config  *Config
	Root    string
	Service string
	Stage   string
	Region  string
	Mode    string
	Journal any
}

// RecoveryResult contains provider recovery outcome.
type RecoveryResult struct {
	Recovered  bool              `json:"recovered"`
	Mode       string            `json:"mode"`
	Status     string            `json:"status"`
	Message    string            `json:"message,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	Errors     []string          `json:"errors,omitempty"`
	ResumeData map[string]any    `json:"resumeData,omitempty"`
}

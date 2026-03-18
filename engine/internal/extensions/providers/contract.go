// Package providers defines the provider plugin contract and result types for RunFabric.
// Built-in implementations (AWS, GCP, etc.) live under engine/internal/extensions/provider/<name> and are
// registered via app.Bootstrap or Registry.RegisterPlugin.
package providers

import (
	"context"

	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/planner"
)

// Config is the runfabric config type for Provider methods. Re-exported so implementors
// in other packages (e.g. engine/internal/extensions/provider/aws) use the same type and satisfy the interface.
type Config = config.Config

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

// DeployResult is the result of Provider.Deploy.
type DeployResult struct {
	Provider     string            `json:"provider"`
	DeploymentID string            `json:"deploymentId"`
	Outputs      map[string]string `json:"outputs"`
	Artifacts    []Artifact        `json:"artifacts,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
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
	Workflow string `json:"aiWorkflowHash,omitempty"`
}

// LogsResult is the result of Provider.Logs.
type LogsResult struct {
	Provider string   `json:"provider"`
	Function string   `json:"function"`
	Lines    []string `json:"lines"`
	Workflow string   `json:"aiWorkflowHash,omitempty"`
}

// Provider is the legacy interface that provider plugins can implement (narrow, no context/request).
// New plugins should implement ProviderPlugin; use RegisterPlugin to register them.
// Legacy providers implementing Provider are wrapped and registered with Register().
type Provider interface {
	Name() string
	Doctor(cfg *Config, stage string) (*DoctorResult, error)
	Plan(cfg *Config, stage, root string) (*PlanResult, error)
	Deploy(cfg *Config, stage, root string) (*DeployResult, error)
	Remove(cfg *Config, stage, root string) (*RemoveResult, error)
	Invoke(cfg *Config, stage, function string, payload []byte) (*InvokeResult, error)
	Logs(cfg *Config, stage, function string) (*LogsResult, error)
}

// ProviderPlugin is the recommended interface for internal provider plugins (context + request/result).
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

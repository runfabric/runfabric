package providers

import (
	"github.com/runfabric/runfabric/internal/config"
	"github.com/runfabric/runfabric/internal/planner"
)

type DoctorResult struct {
	Provider string   `json:"provider"`
	Checks   []string `json:"checks"`
}

type PlanResult struct {
	Provider string        `json:"provider"`
	Plan     *planner.Plan `json:"plan"`
	Warnings []string      `json:"warnings,omitempty"`
}

type Artifact struct {
	Function        string `json:"function"`
	Runtime         string `json:"runtime"`
	SourcePath      string `json:"sourcePath"`
	OutputPath      string `json:"outputPath"`
	SHA256          string `json:"sha256"`
	SizeBytes       int64  `json:"sizeBytes"`
	ConfigSignature string `json:"configSignature,omitempty"`
}

type DeployResult struct {
	Provider     string            `json:"provider"`
	DeploymentID string            `json:"deploymentId"`
	Outputs      map[string]string `json:"outputs"`
	Artifacts    []Artifact        `json:"artifacts,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

type RemoveResult struct {
	Provider string `json:"provider"`
	Removed  bool   `json:"removed"`
}

type InvokeResult struct {
	Provider string `json:"provider"`
	Function string `json:"function"`
	Output   string `json:"output"`
}

type LogsResult struct {
	Provider string   `json:"provider"`
	Function string   `json:"function"`
	Lines    []string `json:"lines"`
}

type Provider interface {
	Name() string
	Doctor(cfg *config.Config, stage string) (*DoctorResult, error)
	Plan(cfg *config.Config, stage, root string) (*PlanResult, error)
	Deploy(cfg *config.Config, stage, root string) (*DeployResult, error)
	Remove(cfg *config.Config, stage, root string) (*RemoveResult, error)
	Invoke(cfg *config.Config, stage, function string, payload []byte) (*InvokeResult, error)
	Logs(cfg *config.Config, stage, function string) (*LogsResult, error)
}

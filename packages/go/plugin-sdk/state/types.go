package state

import (
	"context"
	"time"

	sdkrouter "github.com/runfabric/runfabric/plugin-sdk/go/router"
)

// FunctionDeployment captures function deployment metadata for state receipts.
type FunctionDeployment struct {
	Function           string            `json:"function"`
	ArtifactSHA256     string            `json:"artifactSha256"`
	ConfigSignature    string            `json:"configSignature"`
	ResourceName       string            `json:"resourceName,omitempty"`
	ResourceIdentifier string            `json:"resourceIdentifier,omitempty"`
	Metadata           map[string]string `json:"metadata,omitempty"`
	EnvironmentHash    string            `json:"environmentHash,omitempty"`
	TagsHash           string            `json:"tagsHash,omitempty"`
	LayersHash         string            `json:"layersHash,omitempty"`
}

// Artifact describes one built artifact persisted in receipt state.
type Artifact struct {
	Function        string `json:"function"`
	Runtime         string `json:"runtime"`
	SourcePath      string `json:"sourcePath"`
	OutputPath      string `json:"outputPath"`
	SHA256          string `json:"sha256"`
	SizeBytes       int64  `json:"sizeBytes"`
	ConfigSignature string `json:"configSignature,omitempty"`
}

// Receipt is the deploy receipt shape shared across state backends.
type Receipt struct {
	Version      int                  `json:"version"`
	Service      string               `json:"service"`
	Stage        string               `json:"stage"`
	Provider     string               `json:"provider"`
	DeploymentID string               `json:"deploymentId"`
	Outputs      map[string]string    `json:"outputs"`
	Artifacts    []Artifact           `json:"artifacts,omitempty"`
	Metadata     map[string]string    `json:"metadata,omitempty"`
	Functions    []FunctionDeployment `json:"functions,omitempty"`
	UpdatedAt    string               `json:"updatedAt"`
}

// ReleaseEntry identifies one deployed stage and its update timestamp.
type ReleaseEntry struct {
	Stage     string `json:"stage"`
	UpdatedAt string `json:"updatedAt"`
}

// OperationType is the journaled operation kind.
type OperationType string

const (
	OpCreateAPI         OperationType = "create_api"
	OpCreateIntegration OperationType = "create_integration"
	OpCreateRoute       OperationType = "create_route"
	OpCreateLambda      OperationType = "create_lambda"
	OpCreateRole        OperationType = "create_role"
	OpCreateFunctionURL OperationType = "create_function_url"
)

// Status is the lifecycle status for journals.
type Status string

const (
	StatusActive      Status = "active"
	StatusRollingBack Status = "rolling_back"
	StatusRolledBack  Status = "rolled_back"
	StatusCompleted   Status = "completed"
	StatusArchived    Status = "archived"
)

// JournalCheckpoint records a durable checkpoint.
type JournalCheckpoint struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// Operation is one journaled operation entry.
type Operation struct {
	Type     OperationType     `json:"type"`
	Resource string            `json:"resource"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// JournalFile is the journal persistence shape shared across backends.
type JournalFile struct {
	Service       string              `json:"service"`
	Stage         string              `json:"stage"`
	Operation     string              `json:"operation"`
	Status        Status              `json:"status"`
	StartedAt     string              `json:"startedAt"`
	UpdatedAt     string              `json:"updatedAt"`
	Version       int                 `json:"version"`
	AttemptCount  int                 `json:"attemptCount"`
	LastAttemptAt string              `json:"lastAttemptAt,omitempty"`
	Checksum      string              `json:"checksum,omitempty"`
	Checkpoints   []JournalCheckpoint `json:"checkpoints,omitempty"`
	Operations    []Operation         `json:"operations"`
}

// LockRecord captures lock owner/state metadata.
type LockRecord struct {
	Service         string `json:"service"`
	Stage           string `json:"stage"`
	Operation       string `json:"operation"`
	OwnerToken      string `json:"ownerToken"`
	PID             int    `json:"pid"`
	CreatedAt       string `json:"createdAt"`
	ExpiresAt       string `json:"expiresAt"`
	LastHeartbeatAt string `json:"lastHeartbeatAt,omitempty"`
}

// Plugin is the interface all state plugins implement.
type Plugin interface {
	Meta() sdkrouter.PluginMeta
	LockAcquire(ctx context.Context, service, stage, operation string, staleAfterMillis int64) (*LockRecord, error)
	LockRead(ctx context.Context, service, stage string) (*LockRecord, error)
	LockRelease(ctx context.Context, service, stage string) error
	JournalLoad(ctx context.Context, service, stage string) (*JournalFile, error)
	JournalSave(ctx context.Context, journal *JournalFile) error
	JournalDelete(ctx context.Context, service, stage string) error
	ReceiptLoad(ctx context.Context, stage string) (*Receipt, error)
	ReceiptSave(ctx context.Context, receipt *Receipt) error
	ReceiptDelete(ctx context.Context, stage string) error
	ReceiptListReleases(ctx context.Context) ([]ReleaseEntry, error)
}

// Registry stores state plugins.
type Registry interface {
	Get(id string) (Plugin, error)
	Register(state Plugin) error
}

// MillisToDuration converts stale-after milliseconds to a duration.
func MillisToDuration(ms int64) time.Duration {
	if ms <= 0 {
		return 0
	}
	return time.Duration(ms) * time.Millisecond
}

package types

import "time"

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

// CheckResult is a minimal health/doctor check result shape.
type CheckResult struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Backend string `json:"backend,omitempty"`
	Message string `json:"message,omitempty"`
}

// ConflictError represents optimistic-write conflicts for journal/state updates.
type ConflictError struct {
	Backend         string
	Service         string
	Stage           string
	Resource        string
	CurrentVersion  int
	IncomingVersion int
	Action          string
}

func (e *ConflictError) Error() string {
	return "journal conflict backend=" + e.Backend +
		" service=" + e.Service +
		" stage=" + e.Stage +
		" resource=" + e.Resource +
		" current=" + itoa(e.CurrentVersion) +
		" incoming=" + itoa(e.IncomingVersion) +
		"; action=" + e.Action
}

// Handle is a backend-agnostic lock handle.
type Handle struct {
	Service    string
	Stage      string
	OwnerToken string
	Held       bool
	r          Releaser
	n          Renewer
	or         OwnedReleaser
}

// Releaser releases a lock by service/stage.
type Releaser interface {
	Release(service, stage string) error
}

// Renewer renews a lock lease.
type Renewer interface {
	Renew(service, stage, ownerToken string, leaseFor time.Duration) error
}

// OwnedReleaser releases a lock if the caller owns the token.
type OwnedReleaser interface {
	ReleaseOwned(service, stage, ownerToken string) error
}

// NewHandle constructs a lock handle with optional renew/release-owned support.
func NewHandle(service, stage, ownerToken string, r Releaser, n Renewer, or OwnedReleaser) *Handle {
	return &Handle{
		Service:    service,
		Stage:      stage,
		OwnerToken: ownerToken,
		Held:       true,
		r:          r,
		n:          n,
		or:         or,
	}
}

// Release releases the lock once; no-op for nil or already-released handles.
func (h *Handle) Release() error {
	if h == nil || !h.Held {
		return nil
	}

	var err error
	if h.or != nil && h.OwnerToken != "" {
		err = h.or.ReleaseOwned(h.Service, h.Stage, h.OwnerToken)
	} else {
		err = h.r.Release(h.Service, h.Stage)
	}
	if err != nil {
		return err
	}

	h.Held = false
	return nil
}

// Renew extends the lock lease when supported.
func (h *Handle) Renew(leaseFor time.Duration) error {
	if h == nil || !h.Held || h.n == nil {
		return nil
	}
	return h.n.Renew(h.Service, h.Stage, h.OwnerToken, leaseFor)
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	buf := [20]byte{}
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + (v % 10))
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

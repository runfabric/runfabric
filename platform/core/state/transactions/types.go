package transactions

type OperationType string

const (
	OpCreateAPI         OperationType = "create_api"
	OpCreateIntegration OperationType = "create_integration"
	OpCreateRoute       OperationType = "create_route"
	OpCreateLambda      OperationType = "create_lambda"
	OpCreateRole        OperationType = "create_role"
	OpCreateFunctionURL OperationType = "create_function_url"
)

type Status string

const (
	StatusActive      Status = "active"
	StatusRollingBack Status = "rolling_back"
	StatusRolledBack  Status = "rolled_back"
	StatusCompleted   Status = "completed"
	StatusArchived    Status = "archived"
)

type JournalCheckpoint struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type Operation struct {
	Type     OperationType     `json:"type"`
	Resource string            `json:"resource"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

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

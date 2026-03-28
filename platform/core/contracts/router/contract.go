package router

import (
	"context"
	"io"
)

// PluginMeta identifies a router implementation.
type PluginMeta struct {
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	Version     string `json:"version,omitempty"`
	Description string `json:"description,omitempty"`
}

// RoutingEndpoint is a single backend target.
type RoutingEndpoint struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	Healthy *bool  `json:"healthy,omitempty"`
	Weight  int    `json:"weight,omitempty"`
}

// RoutingConfig is the abstract routing manifest for a service stage.
type RoutingConfig struct {
	Contract   string            `json:"contract"`
	Service    string            `json:"service"`
	Stage      string            `json:"stage"`
	Hostname   string            `json:"hostname"`
	Strategy   string            `json:"strategy"`
	HealthPath string            `json:"healthPath,omitempty"`
	TTL        int               `json:"ttl,omitempty"`
	Endpoints  []RoutingEndpoint `json:"endpoints"`
}

// SyncAction describes a single resource change performed during a sync operation.
type SyncAction struct {
	Resource string `json:"resource"`
	Action   string `json:"action"`
	Name     string `json:"name"`
	Detail   string `json:"detail,omitempty"`
}

// SyncRequest is the input to Router.Sync.
type SyncRequest struct {
	Routing   *RoutingConfig
	ZoneID    string
	AccountID string
	DryRun    bool
	Out       io.Writer
}

// SyncResult is the output of Router.Sync.
type SyncResult struct {
	DryRun  bool         `json:"dryRun"`
	Actions []SyncAction `json:"actions"`
}

// Router is the router contract used by engine DNS sync flows.
type Router interface {
	Meta() PluginMeta
	Sync(ctx context.Context, req SyncRequest) (*SyncResult, error)
}

// Registry stores router implementations.
type Registry interface {
	Get(id string) (Router, error)
	Register(router Router) error
}

package router

import (
	"context"
	"io"
)

// PluginMeta identifies a router plugin.
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

// RouterSyncAction describes a single resource change performed during a sync operation.
type RouterSyncAction struct {
	Resource string `json:"resource"`
	Action   string `json:"action"`
	Name     string `json:"name"`
	Detail   string `json:"detail,omitempty"`
}

// RouterSyncRequest is the input to a Router.Sync call.
type RouterSyncRequest struct {
	Routing   *RoutingConfig
	ZoneID    string
	AccountID string
	DryRun    bool
	Out       io.Writer
}

// RouterSyncResult is the output of a Router.Sync call.
type RouterSyncResult struct {
	DryRun  bool               `json:"dryRun"`
	Actions []RouterSyncAction `json:"actions"`
}

// Router is the interface that all router plugins must implement.
type Router interface {
	Meta() PluginMeta
	Sync(ctx context.Context, req RouterSyncRequest) (*RouterSyncResult, error)
}

// RouterRegistry is the management interface for registered router plugins.
type RouterRegistry interface {
	Get(id string) (Router, error)
	Register(router Router) error
}

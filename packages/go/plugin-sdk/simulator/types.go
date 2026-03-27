package simulator

import (
	"context"
	"encoding/json"

	sdkrouter "github.com/runfabric/runfabric/plugin-sdk/go/router"
)

// Request captures local simulation invocation context.
type Request struct {
	Service    string            `json:"service,omitempty"`
	Stage      string            `json:"stage,omitempty"`
	Function   string            `json:"function,omitempty"`
	Method     string            `json:"method,omitempty"`
	Path       string            `json:"path,omitempty"`
	Query      map[string]string `json:"query,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       []byte            `json:"body,omitempty"`
	WorkDir    string            `json:"workDir,omitempty"`
	HandlerRef string            `json:"handlerRef,omitempty"`
	Runtime    string            `json:"runtime,omitempty"`
}

// Response captures simulator output.
type Response struct {
	StatusCode int               `json:"statusCode"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       json.RawMessage   `json:"body,omitempty"`
}

// PluginMeta identifies a simulator plugin.
type PluginMeta = sdkrouter.PluginMeta

// Plugin is the interface all simulator plugins implement.
type Plugin interface {
	Meta() PluginMeta
	Simulate(ctx context.Context, req Request) (*Response, error)
}

// Registry stores simulator plugins.
type Registry interface {
	Get(id string) (Plugin, error)
	Register(sim Plugin) error
}

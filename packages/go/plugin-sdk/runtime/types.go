package runtime

import (
	"context"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
	sdkrouter "github.com/runfabric/runfabric/plugin-sdk/go/router"
)

// PluginMeta identifies a runtime plugin.
type PluginMeta = sdkrouter.PluginMeta

// FunctionSpec describes a function handler and runtime.
type FunctionSpec struct {
	Handler string `json:"handler"`
	Runtime string `json:"runtime,omitempty"`
}

// BuildRequest is passed to runtime Build implementations.
type BuildRequest struct {
	Root            string
	FunctionName    string
	Function        FunctionSpec
	ConfigSignature string
}

// InvokeRequest is passed to runtime Invoke implementations.
type InvokeRequest struct {
	Root         string
	FunctionName string
	Function     FunctionSpec
	Payload      []byte
}

// InvokeResult is returned by runtime Invoke implementations.
type InvokeResult struct {
	Output []byte            `json:"output,omitempty"`
	Meta   map[string]string `json:"meta,omitempty"`
}

// Plugin is the interface all runtime plugins implement.
type Plugin interface {
	Meta() PluginMeta
	Build(ctx context.Context, req BuildRequest) (*sdkprovider.Artifact, error)
	Invoke(ctx context.Context, req InvokeRequest) (*InvokeResult, error)
}

// Registry stores runtime plugins.
type Registry interface {
	Get(id string) (Plugin, error)
	Register(rt Plugin) error
}

package runtimes

import (
	"context"

	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/extensions/providers"
)

// Meta describes a runtime plugin implementation.
type Meta struct {
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	Version     string `json:"version,omitempty"`
	Description string `json:"description,omitempty"`
}

// BuildRequest is the runtime build input for one function.
type BuildRequest struct {
	Root            string
	FunctionName    string
	FunctionConfig  config.FunctionConfig
	ConfigSignature string
}

// InvokeRequest is the local runtime invoke input for one function.
type InvokeRequest struct {
	Root           string
	FunctionName   string
	FunctionConfig config.FunctionConfig
	Payload        []byte
}

// InvokeResult is the local runtime invoke output.
type InvokeResult struct {
	Output []byte            `json:"output,omitempty"`
	Meta   map[string]string `json:"meta,omitempty"`
}

// Runtime is the runtime plugin contract used by build/local invoke paths.
type Runtime interface {
	Meta() Meta
	Build(ctx context.Context, req BuildRequest) (*providers.Artifact, error)
	Invoke(ctx context.Context, req InvokeRequest) (*InvokeResult, error)
}

package runtimes

import (
	"context"

	extproviders "github.com/runfabric/runfabric/internal/provider/contracts"
	"github.com/runfabric/runfabric/platform/core/model/config"
)

type (
	// Meta describes a runtime plugin implementation.
	Meta struct {
		ID          string `json:"id"`
		Name        string `json:"name,omitempty"`
		Version     string `json:"version,omitempty"`
		Description string `json:"description,omitempty"`
	}

	// BuildRequest is the runtime build input for one function.
	BuildRequest struct {
		Root            string
		FunctionName    string
		FunctionConfig  config.FunctionConfig
		ConfigSignature string
	}

	// InvokeRequest is the local runtime invoke input for one function.
	InvokeRequest struct {
		Root           string
		FunctionName   string
		FunctionConfig config.FunctionConfig
		Payload        []byte
	}

	// InvokeResult is the local runtime invoke output.
	InvokeResult struct {
		Output []byte            `json:"output,omitempty"`
		Meta   map[string]string `json:"meta,omitempty"`
	}
)

// Runtime is the runtime plugin contract used by build/local invoke paths.
type Runtime interface {
	Meta() Meta
	Build(ctx context.Context, req BuildRequest) (*extproviders.Artifact, error)
	Invoke(ctx context.Context, req InvokeRequest) (*InvokeResult, error)
}

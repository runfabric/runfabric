package external

import (
	"context"
	"fmt"
	"strings"

	extproviders "github.com/runfabric/runfabric/internal/provider/contracts"
	runtimecontracts "github.com/runfabric/runfabric/platform/core/contracts/runtime"
)

// ExternalRuntimeAdapter implements runtimecontracts.Runtime over the external
// plugin stdio protocol.
type ExternalRuntimeAdapter struct {
	id     string
	meta   runtimecontracts.Meta
	client *ExternalProviderAdapter
}

func NewExternalRuntimeAdapter(id, executable string, meta runtimecontracts.Meta) *ExternalRuntimeAdapter {
	normalizedID := strings.TrimSpace(id)
	if strings.TrimSpace(meta.ID) == "" {
		meta.ID = normalizedID
	}
	clientMeta := extproviders.ProviderMeta{Name: normalizedID}
	return &ExternalRuntimeAdapter{
		id:     normalizedID,
		meta:   meta,
		client: NewExternalProviderAdapter(normalizedID, executable, clientMeta),
	}
}

func (r *ExternalRuntimeAdapter) Meta() runtimecontracts.Meta {
	meta := r.meta
	if strings.TrimSpace(meta.ID) == "" {
		meta.ID = strings.TrimSpace(r.id)
	}
	return meta
}

func (r *ExternalRuntimeAdapter) Build(ctx context.Context, req runtimecontracts.BuildRequest) (*extproviders.Artifact, error) {
	_ = ctx // Request/response over stdio currently does not carry cancellation.
	var out extproviders.Artifact
	err := r.client.call("Build", map[string]any{
		"root":         req.Root,
		"functionName": req.FunctionName,
		"function": map[string]any{
			"handler": req.FunctionConfig.Handler,
			"runtime": req.FunctionConfig.Runtime,
		},
		"configSignature": req.ConfigSignature,
	}, &out)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(out.Function) == "" {
		out.Function = req.FunctionName
	}
	if strings.TrimSpace(out.Runtime) == "" {
		out.Runtime = req.FunctionConfig.Runtime
	}
	return &out, nil
}

func (r *ExternalRuntimeAdapter) Invoke(ctx context.Context, req runtimecontracts.InvokeRequest) (*runtimecontracts.InvokeResult, error) {
	_ = ctx // Request/response over stdio currently does not carry cancellation.
	var out runtimecontracts.InvokeResult
	err := r.client.call("Invoke", map[string]any{
		"root":         req.Root,
		"functionName": req.FunctionName,
		"function": map[string]any{
			"handler": req.FunctionConfig.Handler,
			"runtime": req.FunctionConfig.Runtime,
		},
		"payload": req.Payload,
	}, &out)
	if err != nil {
		return nil, err
	}
	if out.Output == nil {
		return nil, fmt.Errorf("runtime plugin %q returned empty invoke output", r.id)
	}
	return &out, nil
}

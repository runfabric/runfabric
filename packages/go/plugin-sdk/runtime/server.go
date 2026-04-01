package runtime

import (
	"context"
	"encoding/json"
	"strings"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
	sdkrouter "github.com/runfabric/runfabric/plugin-sdk/go/router"
	"github.com/runfabric/runfabric/plugin-sdk/go/server"
)

// ServeOptions configures the runtime plugin server.
type ServeOptions struct {
	ProtocolVersion string
	Version         string
	Platform        string
}

// NewServer adapts a typed runtime Plugin into the low-level plugin SDK server.
func NewServer(plugin Plugin, opts ServeOptions) *server.Server {
	meta := plugin.Meta()
	handshake := server.HandshakeMetadata{
		Version:         firstNonEmpty(strings.TrimSpace(opts.Version), strings.TrimSpace(meta.Version)),
		Platform:        strings.TrimSpace(opts.Platform),
		Capabilities:    []string{"build", "invoke"},
		SupportsRuntime: deriveSupportsRuntime(meta),
	}

	methods := map[string]server.MethodFunc{
		"Build": func(ctx context.Context, params json.RawMessage) (any, error) {
			var raw struct {
				Root         string `json:"root"`
				FunctionName string `json:"functionName"`
				Function     struct {
					Handler string `json:"handler"`
					Runtime string `json:"runtime,omitempty"`
				} `json:"function"`
				ConfigSignature string `json:"configSignature"`
			}
			if err := decodeParams(params, &raw); err != nil {
				return nil, err
			}
			req := BuildRequest{
				Root:         raw.Root,
				FunctionName: raw.FunctionName,
				Function: FunctionSpec{
					Handler: raw.Function.Handler,
					Runtime: raw.Function.Runtime,
				},
				ConfigSignature: raw.ConfigSignature,
			}
			result, err := plugin.Build(ctx, req)
			if err != nil {
				return nil, err
			}
			if result == nil {
				return &sdkprovider.Artifact{}, nil
			}
			return result, nil
		},
		"Invoke": func(ctx context.Context, params json.RawMessage) (any, error) {
			var raw struct {
				Root         string `json:"root"`
				FunctionName string `json:"functionName"`
				Function     struct {
					Handler string `json:"handler"`
					Runtime string `json:"runtime,omitempty"`
				} `json:"function"`
				Payload []byte `json:"payload"`
			}
			if err := decodeParams(params, &raw); err != nil {
				return nil, err
			}
			req := InvokeRequest{
				Root:         raw.Root,
				FunctionName: raw.FunctionName,
				Function: FunctionSpec{
					Handler: raw.Function.Handler,
					Runtime: raw.Function.Runtime,
				},
				Payload: raw.Payload,
			}
			result, err := plugin.Invoke(ctx, req)
			if err != nil {
				return nil, err
			}
			if result == nil {
				return &InvokeResult{}, nil
			}
			return result, nil
		},
	}

	return server.New(server.Options{
		ProtocolVersion: firstNonEmpty(strings.TrimSpace(opts.ProtocolVersion), "1"),
		Handshake:       handshake,
		Methods:         methods,
	})
}

func decodeParams(raw json.RawMessage, out any) error {
	s := strings.TrimSpace(string(raw))
	if len(raw) == 0 || s == "" || s == "null" {
		return nil
	}
	return json.Unmarshal(raw, out)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func deriveSupportsRuntime(meta sdkrouter.PluginMeta) []string {
	id := strings.ToLower(strings.TrimSpace(meta.ID))
	if id == "" {
		return nil
	}
	return []string{id}
}

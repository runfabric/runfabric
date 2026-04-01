package simulator

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/runfabric/runfabric/plugin-sdk/go/server"
)

// ServeOptions configures the simulator plugin server.
type ServeOptions struct {
	ProtocolVersion string
	Version         string
	Platform        string
}

// NewServer adapts a typed simulator Plugin into the low-level plugin SDK server.
func NewServer(plugin Plugin, opts ServeOptions) *server.Server {
	meta := plugin.Meta()
	handshake := server.HandshakeMetadata{
		Version:      firstNonEmpty(strings.TrimSpace(opts.Version), strings.TrimSpace(meta.Version)),
		Platform:     strings.TrimSpace(opts.Platform),
		Capabilities: []string{"simulate"},
	}

	methods := map[string]server.MethodFunc{
		"Simulate": func(ctx context.Context, params json.RawMessage) (any, error) {
			var req Request
			if err := decodeParams(params, &req); err != nil {
				return nil, err
			}
			result, err := plugin.Simulate(ctx, req)
			if err != nil {
				return nil, err
			}
			if result == nil {
				return &Response{}, nil
			}
			return result, nil
		},
	}

	_ = meta // used for handshake above

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

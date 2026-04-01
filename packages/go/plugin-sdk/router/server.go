package router

import (
	"context"
	"encoding/json"
	"io"
	"strings"

	"github.com/runfabric/runfabric/plugin-sdk/go/server"
)

// ServeOptions configures the router plugin server.
type ServeOptions struct {
	ProtocolVersion string
	Version         string
	Platform        string
}

// NewServer adapts a typed Router into the low-level plugin SDK server.
// The Out field of RouterSyncRequest is always set to io.Discard when served over stdio
// (no streaming output over the wire protocol).
func NewServer(plugin Router, opts ServeOptions) *server.Server {
	meta := plugin.Meta()
	handshake := server.HandshakeMetadata{
		Version:      firstNonEmpty(strings.TrimSpace(opts.Version), strings.TrimSpace(meta.Version)),
		Platform:     strings.TrimSpace(opts.Platform),
		Capabilities: []string{"sync"},
	}

	methods := map[string]server.MethodFunc{
		"Sync": func(ctx context.Context, params json.RawMessage) (any, error) {
			var raw struct {
				Routing   *RoutingConfig `json:"routing"`
				ZoneID    string         `json:"zoneID"`
				AccountID string         `json:"accountID"`
				DryRun    bool           `json:"dryRun"`
			}
			if err := decodeParams(params, &raw); err != nil {
				return nil, err
			}
			req := RouterSyncRequest{
				Routing:   raw.Routing,
				ZoneID:    raw.ZoneID,
				AccountID: raw.AccountID,
				DryRun:    raw.DryRun,
				Out:       io.Discard,
			}
			result, err := plugin.Sync(ctx, req)
			if err != nil {
				return nil, err
			}
			if result == nil {
				return map[string]any{}, nil
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

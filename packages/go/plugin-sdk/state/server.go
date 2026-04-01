package state

import (
	"context"
	"encoding/json"
	"strings"

	sdkrouter "github.com/runfabric/runfabric/plugin-sdk/go/router"
	"github.com/runfabric/runfabric/plugin-sdk/go/server"
)

// ServeOptions configures the state plugin server.
type ServeOptions struct {
	ProtocolVersion string
	Version         string
	Platform        string
}

// NewServer adapts a typed state Plugin into the low-level plugin SDK server.
func NewServer(plugin Plugin, opts ServeOptions) *server.Server {
	meta := plugin.Meta()
	handshake := server.HandshakeMetadata{
		Version:      firstNonEmpty(strings.TrimSpace(opts.Version), strings.TrimSpace(meta.Version)),
		Platform:     strings.TrimSpace(opts.Platform),
		Capabilities: deriveCapabilities(meta),
	}

	methods := map[string]server.MethodFunc{
		"LockAcquire": func(ctx context.Context, params json.RawMessage) (any, error) {
			var req struct {
				Service          string `json:"service"`
				Stage            string `json:"stage"`
				Operation        string `json:"operation"`
				StaleAfterMillis int64  `json:"staleAfterMillis"`
			}
			if err := decodeParams(params, &req); err != nil {
				return nil, err
			}
			result, err := plugin.LockAcquire(ctx, req.Service, req.Stage, req.Operation, req.StaleAfterMillis)
			if err != nil {
				return nil, err
			}
			if result == nil {
				return &LockRecord{}, nil
			}
			return result, nil
		},
		"LockRead": func(ctx context.Context, params json.RawMessage) (any, error) {
			var req struct {
				Service string `json:"service"`
				Stage   string `json:"stage"`
			}
			if err := decodeParams(params, &req); err != nil {
				return nil, err
			}
			result, err := plugin.LockRead(ctx, req.Service, req.Stage)
			if err != nil {
				return nil, err
			}
			if result == nil {
				return &LockRecord{}, nil
			}
			return result, nil
		},
		"LockRelease": func(ctx context.Context, params json.RawMessage) (any, error) {
			var req struct {
				Service string `json:"service"`
				Stage   string `json:"stage"`
			}
			if err := decodeParams(params, &req); err != nil {
				return nil, err
			}
			if err := plugin.LockRelease(ctx, req.Service, req.Stage); err != nil {
				return nil, err
			}
			return map[string]any{"ok": true}, nil
		},
		"JournalLoad": func(ctx context.Context, params json.RawMessage) (any, error) {
			var req struct {
				Service string `json:"service"`
				Stage   string `json:"stage"`
			}
			if err := decodeParams(params, &req); err != nil {
				return nil, err
			}
			result, err := plugin.JournalLoad(ctx, req.Service, req.Stage)
			if err != nil {
				return nil, err
			}
			if result == nil {
				return &JournalFile{}, nil
			}
			return result, nil
		},
		"JournalSave": func(ctx context.Context, params json.RawMessage) (any, error) {
			var req struct {
				Journal *JournalFile `json:"journal"`
			}
			if err := decodeParams(params, &req); err != nil {
				return nil, err
			}
			if err := plugin.JournalSave(ctx, req.Journal); err != nil {
				return nil, err
			}
			return map[string]any{"ok": true}, nil
		},
		"JournalDelete": func(ctx context.Context, params json.RawMessage) (any, error) {
			var req struct {
				Service string `json:"service"`
				Stage   string `json:"stage"`
			}
			if err := decodeParams(params, &req); err != nil {
				return nil, err
			}
			if err := plugin.JournalDelete(ctx, req.Service, req.Stage); err != nil {
				return nil, err
			}
			return map[string]any{"ok": true}, nil
		},
		"ReceiptLoad": func(ctx context.Context, params json.RawMessage) (any, error) {
			var req struct {
				Stage string `json:"stage"`
			}
			if err := decodeParams(params, &req); err != nil {
				return nil, err
			}
			result, err := plugin.ReceiptLoad(ctx, req.Stage)
			if err != nil {
				return nil, err
			}
			if result == nil {
				return &Receipt{}, nil
			}
			return result, nil
		},
		"ReceiptSave": func(ctx context.Context, params json.RawMessage) (any, error) {
			var req struct {
				Receipt *Receipt `json:"receipt"`
			}
			if err := decodeParams(params, &req); err != nil {
				return nil, err
			}
			if err := plugin.ReceiptSave(ctx, req.Receipt); err != nil {
				return nil, err
			}
			return map[string]any{"ok": true}, nil
		},
		"ReceiptDelete": func(ctx context.Context, params json.RawMessage) (any, error) {
			var req struct {
				Stage string `json:"stage"`
			}
			if err := decodeParams(params, &req); err != nil {
				return nil, err
			}
			if err := plugin.ReceiptDelete(ctx, req.Stage); err != nil {
				return nil, err
			}
			return map[string]any{"ok": true}, nil
		},
		"ReceiptListReleases": func(ctx context.Context, params json.RawMessage) (any, error) {
			var req map[string]any
			if err := decodeParams(params, &req); err != nil {
				return nil, err
			}
			_ = req
			result, err := plugin.ReceiptListReleases(ctx)
			if err != nil {
				return nil, err
			}
			if result == nil {
				return []ReleaseEntry{}, nil
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

func deriveCapabilities(meta sdkrouter.PluginMeta) []string {
	id := strings.ToLower(strings.TrimSpace(meta.ID))
	capabilities := []string{"lock", "journal", "receipt"}
	if id == "" {
		return capabilities
	}
	return append(capabilities, "state:"+id, "backend:"+id)
}

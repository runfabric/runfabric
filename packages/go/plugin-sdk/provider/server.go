package provider

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/runfabric/runfabric/plugin-sdk/go/server"
)

type ServeOptions struct {
	ProtocolVersion string
	Version         string
	Platform        string
}

// NewServer adapts a typed provider Plugin into the low-level plugin SDK server.
// This keeps plugin implementations strongly typed while preserving the JSON wire protocol.
func NewServer(plugin Plugin, opts ServeOptions) *server.Server {
	meta := plugin.Meta()
	handshake := server.HandshakeMetadata{
		Version:           firstNonEmpty(strings.TrimSpace(opts.Version), strings.TrimSpace(meta.Version)),
		Platform:          strings.TrimSpace(opts.Platform),
		Capabilities:      deriveCapabilities(plugin, meta),
		SupportsRuntime:   append([]string(nil), meta.SupportsRuntime...),
		SupportsTriggers:  append([]string(nil), meta.SupportsTriggers...),
		SupportsResources: append([]string(nil), meta.SupportsResources...),
	}

	methods := map[string]server.MethodFunc{
		"ValidateConfig": bind(plugin.ValidateConfig),
		"Doctor":         bindResult(plugin.Doctor),
		"Plan":           bindResult(plugin.Plan),
		"Deploy":         bindResult(plugin.Deploy),
		"Remove":         bindResult(plugin.Remove),
		"Invoke":         bindResult(plugin.Invoke),
		"Logs":           bindResult(plugin.Logs),
	}

	if obs, ok := plugin.(ObservabilityCapable); ok {
		methods["FetchMetrics"] = bindResult(obs.FetchMetrics)
		methods["FetchTraces"] = bindResult(obs.FetchTraces)
	}
	if ds, ok := plugin.(DevStreamCapable); ok {
		methods["PrepareDevStream"] = bindResult(ds.PrepareDevStream)
	}
	if rec, ok := plugin.(RecoveryCapable); ok {
		methods["Recover"] = bindResult(rec.Recover)
	}
	if orch, ok := plugin.(OrchestrationCapable); ok {
		methods["SyncOrchestrations"] = bindResult(orch.SyncOrchestrations)
		methods["RemoveOrchestrations"] = bindResult(orch.RemoveOrchestrations)
		methods["InvokeOrchestration"] = bindResult(orch.InvokeOrchestration)
		methods["InspectOrchestrations"] = bindValueResult(orch.InspectOrchestrations)
	}

	return server.New(server.Options{
		ProtocolVersion: firstNonEmpty(strings.TrimSpace(opts.ProtocolVersion), "1"),
		Handshake:       handshake,
		Methods:         methods,
	})
}

func bind[T any](fn func(context.Context, T) error) server.MethodFunc {
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		var req T
		if err := decodeParams(params, &req); err != nil {
			return nil, err
		}
		if err := fn(ctx, req); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true}, nil
	}
}

func bindResult[T any, R any](fn func(context.Context, T) (*R, error)) server.MethodFunc {
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		var req T
		if err := decodeParams(params, &req); err != nil {
			return nil, err
		}
		out, err := fn(ctx, req)
		if err != nil {
			return nil, err
		}
		if out == nil {
			return map[string]any{}, nil
		}
		return out, nil
	}
}

func bindValueResult[T any, R any](fn func(context.Context, T) (R, error)) server.MethodFunc {
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		var req T
		if err := decodeParams(params, &req); err != nil {
			return nil, err
		}
		out, err := fn(ctx, req)
		if err != nil {
			return nil, err
		}
		return out, nil
	}
}

func decodeParams(raw json.RawMessage, out any) error {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "" || strings.TrimSpace(string(raw)) == "null" {
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

func deriveCapabilities(plugin Plugin, meta Meta) []string {
	// Base provider lifecycle capabilities are always available on Plugin.
	base := []string{"doctor", "plan", "deploy", "remove", "invoke", "logs"}
	set := map[string]struct{}{}
	for _, v := range base {
		set[v] = struct{}{}
	}
	for _, raw := range meta.Capabilities {
		v := strings.ToLower(strings.TrimSpace(raw))
		if v == "" {
			continue
		}
		set[v] = struct{}{}
	}
	if _, ok := plugin.(ObservabilityCapable); ok {
		set["observability"] = struct{}{}
	}
	if _, ok := plugin.(DevStreamCapable); ok {
		set["dev-stream"] = struct{}{}
	}
	if _, ok := plugin.(RecoveryCapable); ok {
		set["recovery"] = struct{}{}
	}
	if _, ok := plugin.(OrchestrationCapable); ok {
		set["orchestration"] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	// Keep stable order for tests and deterministic handshakes.
	order := []string{"doctor", "plan", "deploy", "remove", "invoke", "logs", "observability", "dev-stream", "recovery", "orchestration"}
	ordered := make([]string, 0, len(out))
	for _, k := range order {
		if _, ok := set[k]; ok {
			ordered = append(ordered, k)
			delete(set, k)
		}
	}
	for k := range set {
		ordered = append(ordered, k)
	}
	return ordered
}

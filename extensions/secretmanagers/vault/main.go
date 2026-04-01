package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	sdkserver "github.com/runfabric/runfabric/plugin-sdk/go/server"
)

const (
	pluginVersion     = "0.1.0"
	protocolVersion   = "1"
	defaultCapability = "ResolveSecret"
	envVaultAddr      = "VAULT_ADDR"
	envVaultToken     = "VAULT_TOKEN"
	envVaultNamespace = "VAULT_NAMESPACE"
)

type plugin struct {
	httpClient *http.Client
	getenv     func(string) string
}

type resolveRequest struct {
	Ref string `json:"ref"`
}

type vaultSecretRef struct {
	Address   string
	Namespace string
	Path      string
	Field     string
}

func main() {
	p := newPlugin()
	s := sdkserver.New(sdkserver.Options{
		ProtocolVersion: protocolVersion,
		Handshake: sdkserver.HandshakeMetadata{
			Version:      pluginVersion,
			Platform:     runtime.GOOS + "/" + runtime.GOARCH,
			Capabilities: []string{defaultCapability},
		},
		Methods: map[string]sdkserver.MethodFunc{
			"ResolveSecret": p.resolveSecretMethod,
			"GetSecret":     p.resolveSecretMethod,
		},
	})
	if err := s.Serve(context.Background(), os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newPlugin() *plugin {
	return &plugin{
		httpClient: &http.Client{Timeout: 20 * time.Second},
		getenv:     os.Getenv,
	}
}

func (p *plugin) resolveSecretMethod(ctx context.Context, params json.RawMessage) (any, error) {
	var req resolveRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("decode params: %w", err)
	}
	value, err := p.ResolveSecret(ctx, req.Ref)
	if err != nil {
		return nil, err
	}
	return map[string]any{"value": value}, nil
}

func (p *plugin) ResolveSecret(ctx context.Context, ref string) (string, error) {
	parsed, err := parseVaultSecretRef(ref)
	if err != nil {
		return "", err
	}
	if parsed.Address == "" {
		parsed.Address = strings.TrimSpace(p.getenv(envVaultAddr))
	}
	if parsed.Address == "" {
		return "", fmt.Errorf("vault secret reference %q requires address via ?addr=... or %s", ref, envVaultAddr)
	}
	token := strings.TrimSpace(p.getenv(envVaultToken))
	if token == "" {
		return "", fmt.Errorf("vault secret reference %q requires %s", ref, envVaultToken)
	}
	if parsed.Namespace == "" {
		parsed.Namespace = strings.TrimSpace(p.getenv(envVaultNamespace))
	}

	endpoint := strings.TrimRight(parsed.Address, "/") + "/v1/" + strings.TrimLeft(parsed.Path, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-Vault-Token", token)
	if parsed.Namespace != "" {
		req.Header.Set("X-Vault-Namespace", parsed.Namespace)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("vault request failed for %q: %w", parsed.Path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("vault request failed for %q: %s: %s", parsed.Path, resp.Status, strings.TrimSpace(string(body)))
	}

	value, err := extractVaultValue(body, parsed.Field)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("vault secret reference %q resolved to empty value", ref)
	}
	return value, nil
}

func parseVaultSecretRef(ref string) (*vaultSecretRef, error) {
	trimmed := strings.TrimSpace(ref)
	if !strings.HasPrefix(trimmed, "vault://") {
		return nil, fmt.Errorf("unsupported vault secret reference %q (expected vault://...)", ref)
	}
	raw := strings.TrimPrefix(trimmed, "vault://")
	query := ""
	if idx := strings.Index(raw, "?"); idx >= 0 {
		query = raw[idx+1:]
		raw = raw[:idx]
	}
	raw = strings.Trim(raw, "/")
	if raw == "" {
		return nil, fmt.Errorf("vault secret reference %q has empty path", ref)
	}
	values := parseQueryValues(query)
	return &vaultSecretRef{
		Address:   strings.TrimSpace(values.Get("addr")),
		Namespace: strings.TrimSpace(values.Get("namespace")),
		Path:      raw,
		Field:     strings.TrimSpace(firstNonEmpty(values.Get("field"), values.Get("key"))),
	}, nil
}

func extractVaultValue(raw []byte, field string) (string, error) {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", fmt.Errorf("decode vault response: %w", err)
	}
	data := extractVaultData(payload)
	if data == nil {
		return "", fmt.Errorf("vault response missing data payload")
	}

	if field != "" {
		value, ok := data[field]
		if !ok {
			return "", fmt.Errorf("vault field %q not found in response data", field)
		}
		return scalarToString(value)
	}

	for _, key := range []string{"value", "secret", "data"} {
		if value, ok := data[key]; ok {
			if s, err := scalarToString(value); err == nil {
				return s, nil
			}
		}
	}

	if len(data) == 1 {
		for _, value := range data {
			return scalarToString(value)
		}
	}
	return "", fmt.Errorf("vault response has multiple fields; set ?field=<name>")
}

func extractVaultData(payload map[string]any) map[string]any {
	if payload == nil {
		return nil
	}
	rawData, ok := payload["data"]
	if !ok {
		return nil
	}
	outer, ok := rawData.(map[string]any)
	if !ok {
		return nil
	}
	if innerRaw, ok := outer["data"]; ok {
		if inner, ok := innerRaw.(map[string]any); ok {
			return inner
		}
	}
	return outer
}

func scalarToString(value any) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case float64, float32, int, int32, int64, uint, uint32, uint64, bool:
		return fmt.Sprint(v), nil
	default:
		return "", fmt.Errorf("field value is not scalar")
	}
}

func parseQueryValues(raw string) mapValues {
	values := mapValues{}
	for _, pair := range strings.Split(raw, "&") {
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, "=", 2)
		key := strings.TrimSpace(parts[0])
		if key == "" {
			continue
		}
		value := ""
		if len(parts) == 2 {
			value = strings.TrimSpace(parts[1])
		}
		values[key] = value
	}
	return values
}

type mapValues map[string]string

func (v mapValues) Get(key string) string {
	if v == nil {
		return ""
	}
	return v[key]
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

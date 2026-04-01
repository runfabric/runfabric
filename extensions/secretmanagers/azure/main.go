package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	sdkserver "github.com/runfabric/runfabric/plugin-sdk/go/server"
)

const (
	pluginVersion        = "0.1.0"
	protocolVersion      = "1"
	defaultCapability    = "ResolveSecret"
	envAzureKeyVaultName = "AZURE_KEY_VAULT_NAME"
)

type commandRunner func(ctx context.Context, name string, args ...string) ([]byte, error)

type plugin struct {
	run    commandRunner
	getenv func(string) string
}

type resolveRequest struct {
	Ref string `json:"ref"`
}

type azureSecretRef struct {
	Vault   string
	Secret  string
	Version string
	JSONKey string
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
	return &plugin{run: defaultCommandRunner, getenv: os.Getenv}
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
	parsed, err := parseAzureSecretRef(ref)
	if err != nil {
		return "", err
	}
	if parsed.Vault == "" {
		parsed.Vault = strings.TrimSpace(p.getenv(envAzureKeyVaultName))
	}
	if parsed.Vault == "" {
		return "", fmt.Errorf("azure key vault reference %q requires vault in ref or %s", ref, envAzureKeyVaultName)
	}

	args := []string{
		"keyvault", "secret", "show",
		"--vault-name", parsed.Vault,
		"--name", parsed.Secret,
		"--query", "value",
		"-o", "tsv",
	}
	if parsed.Version != "" {
		args = append(args, "--version", parsed.Version)
	}
	out, err := p.run(ctx, "az", args...)
	if err != nil {
		return "", fmt.Errorf(
			"az keyvault secret show failed for %q (vault=%q, version=%q): %w",
			parsed.Secret,
			parsed.Vault,
			parsed.Version,
			err,
		)
	}
	value := strings.TrimSpace(string(out))
	if value == "" {
		return "", fmt.Errorf("azure key vault reference %q resolved to empty value", ref)
	}
	if parsed.JSONKey != "" {
		return selectJSONKey(value, parsed.JSONKey)
	}
	return value, nil
}

func parseAzureSecretRef(ref string) (*azureSecretRef, error) {
	trimmed := strings.TrimSpace(ref)
	raw := ""
	switch {
	case strings.HasPrefix(trimmed, "azure-kv://"):
		raw = strings.TrimPrefix(trimmed, "azure-kv://")
	case strings.HasPrefix(trimmed, "azure-keyvault://"):
		raw = strings.TrimPrefix(trimmed, "azure-keyvault://")
	default:
		return nil, fmt.Errorf("unsupported azure key vault reference %q (expected azure-kv://...)", ref)
	}

	query := ""
	if idx := strings.Index(raw, "?"); idx >= 0 {
		query = raw[idx+1:]
		raw = raw[:idx]
	}
	raw = strings.Trim(raw, "/")
	parts := splitNonEmpty(raw, "/")
	values := parseQueryValues(query)

	out := &azureSecretRef{
		Vault:   strings.TrimSpace(firstNonEmpty(values.Get("vault"), values.Get("vaultName"))),
		Secret:  strings.TrimSpace(values.Get("secret")),
		Version: strings.TrimSpace(values.Get("version")),
		JSONKey: strings.TrimSpace(values.Get("jsonKey")),
	}
	if out.Vault == "" && len(parts) >= 2 {
		out.Vault = parts[0]
	}
	if out.Secret == "" {
		switch {
		case len(parts) >= 2:
			out.Secret = parts[1]
		case len(parts) == 1:
			out.Secret = parts[0]
		}
	}
	if out.Version == "" && len(parts) >= 3 {
		out.Version = parts[2]
	}
	if out.Secret == "" {
		return nil, fmt.Errorf("azure key vault reference %q has empty secret name", ref)
	}
	return out, nil
}

func defaultCommandRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = os.Environ()
	return cmd.CombinedOutput()
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

func splitNonEmpty(input, sep string) []string {
	rawParts := strings.Split(input, sep)
	out := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func selectJSONKey(raw, key string) (string, error) {
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return "", fmt.Errorf("jsonKey=%q requires JSON secret value: %w", key, err)
	}
	value, ok := payload[key]
	if !ok {
		return "", fmt.Errorf("jsonKey %q not found in Azure Key Vault secret JSON payload", key)
	}
	s, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("jsonKey %q must map to a string value", key)
	}
	return s, nil
}

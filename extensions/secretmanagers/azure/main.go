package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"

	sdkserver "github.com/runfabric/runfabric/plugin-sdk/go/server"
)

const (
	pluginVersion         = "0.1.0"
	protocolVersion       = "1"
	defaultCapability     = "ResolveSecret"
	envAzureKeyVaultName  = "AZURE_KEY_VAULT_NAME"
	envAzureEndpointURL   = "AZURE_ENDPOINT_URL"
	envAzureKeyVaultToken = "AZURE_KEYVAULT_TOKEN"
	keyVaultAPIVersion    = "7.4"
)

type commandRunner func(ctx context.Context, name string, args ...string) ([]byte, error)

type plugin struct {
	run        commandRunner
	getenv     func(string) string
	httpClient *http.Client
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
		},
	})
	if err := s.Serve(context.Background(), os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newPlugin() *plugin {
	return &plugin{run: defaultCommandRunner, getenv: os.Getenv, httpClient: http.DefaultClient}
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

	var raw string
	if base := strings.TrimSpace(p.getenv(envAzureEndpointURL)); base != "" {
		raw, err = p.resolveViaEndpoint(ctx, base, parsed)
	} else {
		raw, err = p.resolveViaCLI(ctx, parsed)
	}
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", fmt.Errorf("azure key vault reference %q resolved to empty value", ref)
	}
	if parsed.JSONKey != "" {
		return selectJSONKey(value, parsed.JSONKey)
	}
	return value, nil
}

// resolveViaCLI reads the secret through the Azure CLI (az keyvault secret show),
// the production path taken when AZURE_ENDPOINT_URL is unset. The CLI carries its
// own authentication (the logged-in principal / managed identity).
func (p *plugin) resolveViaCLI(ctx context.Context, parsed *azureSecretRef) (string, error) {
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
	return string(out), nil
}

// resolveViaEndpoint reads the secret over the Key Vault data-plane REST API
// against the AZURE_ENDPOINT_URL override (e.g. floci-az). Real Azure serves each
// vault at https://<vault>.vault.azure.net/secrets/<name>?api-version=...; when the
// override is set the vault subdomain becomes the first path segment so a
// path-style emulator can route it under one host, and the /secrets/<name> path and
// api-version query are preserved. An optional bearer token from
// AZURE_KEYVAULT_TOKEN is attached when present. Production (override unset) never
// reaches this path.
func (p *plugin) resolveViaEndpoint(ctx context.Context, base string, parsed *azureSecretRef) (string, error) {
	url := keyVaultSecretURL(base, parsed)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("build key vault request %s: %w", url, err)
	}
	if token := strings.TrimSpace(p.getenv(envAzureKeyVaultToken)); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := p.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("key vault request %s: %w", url, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read key vault response from %s: %w", url, err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("key vault request %s: status %d: %s", url, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var payload struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("decode key vault response from %s: %w", url, err)
	}
	return payload.Value, nil
}

// keyVaultSecretURL builds the data-plane GET URL for a secret under the override
// base, preserving the /secrets/<name>[/<version>] path and api-version query.
func keyVaultSecretURL(base string, parsed *azureSecretRef) string {
	url := strings.TrimRight(base, "/") + "/" + parsed.Vault + "/secrets/" + parsed.Secret
	if parsed.Version != "" {
		url += "/" + parsed.Version
	}
	return url + "?api-version=" + keyVaultAPIVersion
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

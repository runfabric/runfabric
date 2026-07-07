package main

import (
	"context"
	"encoding/base64"
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
	pluginVersion     = "0.1.0"
	protocolVersion   = "1"
	defaultCapability = "ResolveSecret"
	envGCPProjectID   = "GCP_PROJECT_ID"
	envGoogleProject  = "GOOGLE_CLOUD_PROJECT"
	// envGCPEndpointURL, when set, points secret access at a local
	// emulator/proxy instead of the real Secret Manager REST API.
	envGCPEndpointURL = "GCP_ENDPOINT_URL"

	// secretManagerBaseURL is the real Secret Manager REST host. It is only
	// contacted directly when envGCPEndpointURL rewrites its scheme+host;
	// otherwise credentials are handled by the gcloud CLI (Application Default
	// Credentials).
	secretManagerBaseURL = "https://secretmanager.googleapis.com"
)

type commandRunner func(ctx context.Context, name string, args ...string) ([]byte, error)

type plugin struct {
	run    commandRunner
	getenv func(string) string
}

type resolveRequest struct {
	Ref string `json:"ref"`
}

type gcpSecretRef struct {
	Project string
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
	parsed, err := parseGCPSecretRef(ref)
	if err != nil {
		return "", err
	}
	if parsed.Project == "" {
		parsed.Project = strings.TrimSpace(p.getenv(envGCPProjectID))
	}
	if parsed.Project == "" {
		parsed.Project = strings.TrimSpace(p.getenv(envGoogleProject))
	}
	if parsed.Project == "" {
		return "", fmt.Errorf("gcp secret reference %q requires project in ref or %s/%s", ref, envGCPProjectID, envGoogleProject)
	}
	if parsed.Version == "" {
		parsed.Version = "latest"
	}

	var value string
	if override := strings.TrimSpace(p.getenv(envGCPEndpointURL)); override != "" {
		value, err = p.accessViaEndpoint(ctx, override, parsed)
	} else {
		value, err = p.accessViaGcloud(ctx, parsed)
	}
	if err != nil {
		return "", err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("gcp secret reference %q resolved to empty value", ref)
	}
	if parsed.JSONKey != "" {
		return selectJSONKey(value, parsed.JSONKey)
	}
	return value, nil
}

// accessViaGcloud resolves a secret through the gcloud CLI, relying on
// Application Default Credentials for authentication.
func (p *plugin) accessViaGcloud(ctx context.Context, ref *gcpSecretRef) (string, error) {
	out, err := p.run(
		ctx,
		"gcloud",
		"secrets", "versions", "access", ref.Version,
		"--secret", ref.Secret,
		"--project", ref.Project,
		"--quiet",
	)
	if err != nil {
		return "", fmt.Errorf("gcloud secret access failed for %q (project=%q, version=%q): %w", ref.Secret, ref.Project, ref.Version, err)
	}
	return string(out), nil
}

// accessViaEndpoint resolves a secret over the Secret Manager REST API with its
// scheme+host rewritten to the override endpoint (a local emulator/proxy). The
// REST path is preserved so the same request shape reaches the override.
func (p *plugin) accessViaEndpoint(ctx context.Context, override string, ref *gcpSecretRef) (string, error) {
	defaultURL := fmt.Sprintf(
		"%s/v1/projects/%s/secrets/%s/versions/%s:access",
		secretManagerBaseURL, ref.Project, ref.Secret, ref.Version,
	)
	target := secretManagerHost(defaultURL, override)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return "", fmt.Errorf("build secret access request for %q: %w", ref.Secret, err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("secret access request to %q failed: %w", target, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read secret access response for %q: %w", ref.Secret, err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("secret access for %q returned status %d: %s", ref.Secret, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload struct {
		Payload struct {
			Data string `json:"data"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("decode secret access response for %q: %w", ref.Secret, err)
	}
	decoded, err := base64.StdEncoding.DecodeString(payload.Payload.Data)
	if err != nil {
		return "", fmt.Errorf("decode secret payload for %q: %w", ref.Secret, err)
	}
	return string(decoded), nil
}

func parseGCPSecretRef(ref string) (*gcpSecretRef, error) {
	trimmed := strings.TrimSpace(ref)
	if !strings.HasPrefix(trimmed, "gcp-sm://") {
		return nil, fmt.Errorf("unsupported gcp secret reference %q (expected gcp-sm://...)", ref)
	}
	raw := strings.TrimPrefix(trimmed, "gcp-sm://")
	query := ""
	if idx := strings.Index(raw, "?"); idx >= 0 {
		query = raw[idx+1:]
		raw = raw[:idx]
	}
	raw = strings.Trim(raw, "/")
	parts := splitNonEmpty(raw, "/")
	values := parseQueryValues(query)

	out := &gcpSecretRef{
		Project: strings.TrimSpace(values.Get("project")),
		Secret:  strings.TrimSpace(values.Get("secret")),
		Version: strings.TrimSpace(values.Get("version")),
		JSONKey: strings.TrimSpace(values.Get("jsonKey")),
	}
	if out.Project == "" && len(parts) >= 2 {
		out.Project = parts[0]
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
		return nil, fmt.Errorf("gcp secret reference %q has empty secret id", ref)
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

func selectJSONKey(raw, key string) (string, error) {
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return "", fmt.Errorf("jsonKey=%q requires JSON secret value: %w", key, err)
	}
	value, ok := payload[key]
	if !ok {
		return "", fmt.Errorf("jsonKey %q not found in GCP secret JSON payload", key)
	}
	s, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("jsonKey %q must map to a string value", key)
	}
	return s, nil
}

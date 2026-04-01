package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
	"unicode/utf8"

	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	sdkserver "github.com/runfabric/runfabric/plugin-sdk/go/server"
)

const (
	pluginVersion     = "0.1.0"
	protocolVersion   = "1"
	defaultAPICap     = "ResolveSecret"
	envAWSRegion      = "AWS_REGION"
	envAWSDefaultZone = "AWS_DEFAULT_REGION"
)

type secretFetcher func(ctx context.Context, region, secretID, versionStage, versionID string) (string, error)

type plugin struct {
	fetch  secretFetcher
	getenv func(string) string
}

type resolveRequest struct {
	Ref string `json:"ref"`
}

type awsSecretRef struct {
	SecretID     string
	Region       string
	VersionStage string
	VersionID    string
	JSONKey      string
}

func main() {
	p := newPlugin()
	s := sdkserver.New(sdkserver.Options{
		ProtocolVersion: protocolVersion,
		Handshake: sdkserver.HandshakeMetadata{
			Version:      pluginVersion,
			Platform:     runtime.GOOS + "/" + runtime.GOARCH,
			Capabilities: []string{defaultAPICap},
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
	return &plugin{fetch: fetchAWSSecretValue, getenv: os.Getenv}
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
	parsed, err := parseAWSSecretRef(ref)
	if err != nil {
		return "", err
	}
	region := strings.TrimSpace(parsed.Region)
	if region == "" {
		region = strings.TrimSpace(p.getenv(envAWSRegion))
	}
	if region == "" {
		region = strings.TrimSpace(p.getenv(envAWSDefaultZone))
	}
	if region == "" {
		return "", fmt.Errorf("aws secret reference %q requires region via ?region=... or %s/%s", ref, envAWSRegion, envAWSDefaultZone)
	}

	raw, err := p.fetch(ctx, region, parsed.SecretID, parsed.VersionStage, parsed.VersionID)
	if err != nil {
		return "", err
	}
	resolved, err := selectSecretValue(raw, parsed.JSONKey)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(resolved) == "" {
		return "", fmt.Errorf("aws secret reference %q resolved to empty value", ref)
	}
	return resolved, nil
}

func parseAWSSecretRef(ref string) (*awsSecretRef, error) {
	trimmed := strings.TrimSpace(ref)
	if !strings.HasPrefix(trimmed, "aws-sm://") {
		return nil, fmt.Errorf("unsupported aws secret reference %q (expected aws-sm://...)", ref)
	}

	query := ""
	secretID := strings.TrimPrefix(trimmed, "aws-sm://")
	if idx := strings.Index(secretID, "?"); idx >= 0 {
		query = secretID[idx+1:]
		secretID = secretID[:idx]
	}
	secretID = strings.TrimSpace(secretID)
	if secretID == "" {
		return nil, fmt.Errorf("aws secret reference %q has empty secret id", ref)
	}

	out := &awsSecretRef{SecretID: secretID}
	if query == "" {
		return out, nil
	}
	values := parseQueryValues(query)
	out.Region = strings.TrimSpace(values.Get("region"))
	out.VersionStage = strings.TrimSpace(values.Get("versionStage"))
	out.VersionID = strings.TrimSpace(values.Get("versionId"))
	out.JSONKey = strings.TrimSpace(values.Get("jsonKey"))
	return out, nil
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

func selectSecretValue(raw, jsonKey string) (string, error) {
	jsonKey = strings.TrimSpace(jsonKey)
	if jsonKey == "" {
		return raw, nil
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return "", fmt.Errorf("jsonKey=%q requires JSON secret value: %w", jsonKey, err)
	}
	value, ok := payload[jsonKey]
	if !ok {
		return "", fmt.Errorf("jsonKey %q not found in AWS secret JSON payload", jsonKey)
	}
	s, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("jsonKey %q must map to a string value", jsonKey)
	}
	return s, nil
}

func fetchAWSSecretValue(ctx context.Context, region, secretID, versionStage, versionID string) (string, error) {
	cfg, err := awscfg.LoadDefaultConfig(ctx, awscfg.WithRegion(region))
	if err != nil {
		return "", fmt.Errorf("load aws config: %w", err)
	}
	client := secretsmanager.NewFromConfig(cfg)
	in := &secretsmanager.GetSecretValueInput{SecretId: &secretID}
	if strings.TrimSpace(versionStage) != "" {
		in.VersionStage = &versionStage
	}
	if strings.TrimSpace(versionID) != "" {
		in.VersionId = &versionID
	}
	out, err := client.GetSecretValue(ctx, in)
	if err != nil {
		return "", fmt.Errorf("aws secretsmanager get-secret-value failed for %q in region %q: %w", secretID, region, err)
	}
	if out.SecretString != nil {
		return *out.SecretString, nil
	}
	if len(out.SecretBinary) > 0 {
		if utf8.Valid(out.SecretBinary) {
			return string(out.SecretBinary), nil
		}
		return base64.StdEncoding.EncodeToString(out.SecretBinary), nil
	}
	return "", fmt.Errorf("aws secret %q has no SecretString or SecretBinary", secretID)
}

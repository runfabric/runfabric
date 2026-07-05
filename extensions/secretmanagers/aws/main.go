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
	awscreds "github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	sdkserver "github.com/runfabric/runfabric/plugin-sdk/go/server"
)

const (
	pluginVersion     = "0.1.0"
	protocolVersion   = "1"
	defaultAPICap     = "ResolveSecret"
	envAWSRegion      = "AWS_REGION"
	envAWSDefaultZone = "AWS_DEFAULT_REGION"

	// Scoped identity for secret reads: when set, these win over the AWS
	// default chain, so secrets can live in a different AWS account than the
	// deploy target (AWS_* / X-Provider-Aws-*) and the state store
	// (RUNFABRIC_STATE_AWS_*). The daemon forwards them per request as
	// X-Secret-Aws-* headers.
	envSMAccessKeyID     = "RUNFABRIC_SM_AWS_ACCESS_KEY_ID"
	envSMSecretAccessKey = "RUNFABRIC_SM_AWS_SECRET_ACCESS_KEY"
	envSMSessionToken    = "RUNFABRIC_SM_AWS_SESSION_TOKEN"
	envSMRegion          = "RUNFABRIC_SM_AWS_REGION"
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
		// Scoped region first: secrets may live in a different account/region
		// than the deploy target's AWS_REGION.
		region = strings.TrimSpace(p.getenv(envSMRegion))
	}
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

// awsLoadOptions builds the SDK load options: the scoped RUNFABRIC_SM_AWS_*
// static identity when set, the default chain otherwise. getenv is injected
// for tests.
func awsLoadOptions(region string, getenv func(string) string) []func(*awscfg.LoadOptions) error {
	opts := []func(*awscfg.LoadOptions) error{awscfg.WithRegion(region)}
	access := strings.TrimSpace(getenv(envSMAccessKeyID))
	secret := strings.TrimSpace(getenv(envSMSecretAccessKey))
	if access != "" && secret != "" {
		opts = append(opts, awscfg.WithCredentialsProvider(
			awscreds.NewStaticCredentialsProvider(access, secret, strings.TrimSpace(getenv(envSMSessionToken)))))
	}
	return opts
}

func fetchAWSSecretValue(ctx context.Context, region, secretID, versionStage, versionID string) (string, error) {
	cfg, err := awscfg.LoadDefaultConfig(ctx, awsLoadOptions(region, os.Getenv)...)
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

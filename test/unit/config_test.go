package unit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/runfabric/runfabric/internal/config"
)

func TestLoadResolveAndValidate(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "runfabric.yml")

	content := `
service: hello-api
provider:
  name: aws
  runtime: nodejs20.x
  region: ${env:AWS_REGION,ap-southeast-1}
functions:
  hello:
    handler: src/handler.hello
`

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	resolved, err := config.Resolve(cfg, "dev")
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}

	if err := config.Validate(resolved); err != nil {
		t.Fatalf("validate failed: %v", err)
	}

	if resolved.Provider.Region != "ap-southeast-1" {
		t.Fatalf("unexpected region: %s", resolved.Provider.Region)
	}
}

func TestValidateBackendKinds(t *testing.T) {
	base := &config.Config{
		Service: "svc",
		Provider: config.ProviderConfig{Name: "aws-lambda", Runtime: "nodejs20.x"},
		Functions: map[string]config.FunctionConfig{
			"fn": {Handler: "handler.handler", Events: []config.EventConfig{}},
		},
	}

	for _, kind := range []string{"local", "s3", "gcs", "azblob", "postgres", "aws-remote"} {
		cfg := *base
		cfg.Backend = &config.BackendConfig{Kind: kind}
		if kind == "s3" || kind == "aws-remote" {
			cfg.Backend.S3Bucket = "my-bucket"
			cfg.Backend.LockTable = "my-lock-table"
		}
		if err := config.Validate(&cfg); err != nil {
			t.Errorf("backend.kind %q should be valid: %v", kind, err)
		}
	}

	// s3 without bucket/table should fail
	cfgBad := *base
	cfgBad.Backend = &config.BackendConfig{Kind: "s3"}
	if err := config.Validate(&cfgBad); err == nil {
		t.Error("backend.kind s3 without s3Bucket/lockTable should fail")
	}

	// unknown kind should fail
	cfgUnknown := *base
	cfgUnknown.Backend = &config.BackendConfig{Kind: "unknown"}
	if err := config.Validate(&cfgUnknown); err == nil {
		t.Error("backend.kind unknown should fail")
	}
}

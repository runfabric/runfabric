package unit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/runfabric/runfabric/platform/core/model/config"
)

func TestLoadResolveAndValidate(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "runfabric.yml")

	content := "service: hello-api\n" +
		"provider:\n" +
		"  name: aws-lambda\n" +
		"  runtime: nodejs20.x\n" +
		"  region: ${env:AWS_REGION,ap-southeast-1}\n" +
		"functions:\n" +
		"  - name: hello\n" +
		"    entry: src/handler.hello\n"

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

func TestLoadRejectsLegacyTopLevelReferenceFields(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "runfabric.yml")
	content := `
service: hello-api
runtime: nodejs
entry: src/index.ts
providers:
  - aws-lambda
triggers:
  - type: http
    method: GET
    path: /hello
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected load to fail for legacy top-level reference fields")
	}
}

func TestResolveDoesNotMutateInput(t *testing.T) {
	cfg := &config.Config{
		Service:  "svc",
		Provider: config.ProviderConfig{Name: "aws", Runtime: "nodejs"},
		Functions: map[string]config.FunctionConfig{
			"api": {
				Handler: "src/handler",
				Events: []config.EventConfig{{
					HTTP: &config.HTTPEvent{Method: "GET", Path: "/hello"},
				}},
			},
		},
	}
	origPath := cfg.Functions["api"].Events[0].HTTP.Path
	resolved, err := config.Resolve(cfg, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Functions["api"].Events[0].HTTP.Path != origPath {
		t.Errorf("resolved path changed to %q", resolved.Functions["api"].Events[0].HTTP.Path)
	}
	if cfg.Functions["api"].Events[0].HTTP.Path != origPath {
		t.Error("Resolve mutated original config: Functions[api].Events[0].HTTP.Path was modified")
	}
}

func TestValidateBackendKinds(t *testing.T) {
	base := &config.Config{
		Service:  "svc",
		Provider: config.ProviderConfig{Name: "aws-lambda", Runtime: "nodejs20.x"},
		Functions: map[string]config.FunctionConfig{
			"fn": {Handler: "handler.handler", Events: []config.EventConfig{}},
		},
	}

	for _, kind := range []string{"local", "s3", "gcs", "azblob", "postgres"} {
		cfg := *base
		cfg.Backend = &config.BackendConfig{Kind: kind}
		if kind == "s3" {
			cfg.Backend.S3Bucket = "my-bucket"
			cfg.Backend.LockTable = "my-lock-table"
		}
		if kind == "gcs" {
			cfg.Backend.GCSBucket = "my-bucket"
			cfg.Backend.GCSPrefix = "runfabric/state"
		}
		if kind == "azblob" {
			cfg.Backend.AzblobContainer = "runfabric-state"
			cfg.Backend.AzblobPrefix = "runfabric/state"
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

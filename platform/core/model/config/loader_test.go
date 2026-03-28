package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_NotFound(t *testing.T) {
	_, err := Load("/nonexistent/runfabric.yml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "runfabric.yml")
	if err := os.WriteFile(path, []byte("invalid: yaml: ["), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoad_MinimalReferenceFunctions(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "runfabric.yml")
	content := "service: svc\n" +
		"provider:\n" +
		"  name: aws-lambda\n" +
		"  runtime: nodejs20.x\n" +
		"functions:\n" +
		"  - name: api\n" +
		"    entry: src/handler.default\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Service != "svc" || cfg.Provider.Name != "aws-lambda" || cfg.Provider.Runtime != "nodejs20.x" {
		t.Errorf("unexpected config: %+v", cfg)
	}
	if len(cfg.Functions) != 1 || cfg.Functions["api"].Handler != "src/handler.default" {
		t.Errorf("unexpected functions: %+v", cfg.Functions)
	}
}

func TestLoadFromBytes_Minimal(t *testing.T) {
	content := []byte("service: svc\n" +
		"provider:\n" +
		"  name: aws-lambda\n" +
		"  runtime: nodejs20.x\n" +
		"functions:\n" +
		"  - name: api\n" +
		"    entry: src/handler.default\n")
	cfg, err := LoadFromBytes(content)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Service != "svc" || cfg.Provider.Name != "aws-lambda" || cfg.Provider.Runtime != "nodejs20.x" {
		t.Errorf("unexpected config: %+v", cfg)
	}
	if len(cfg.Functions) != 1 || cfg.Functions["api"].Handler != "src/handler.default" {
		t.Errorf("unexpected functions: %+v", cfg.Functions)
	}
}

func TestLoadFromBytes_RejectsLegacyWorkflowFields(t *testing.T) {
	content := []byte("service: svc\n" +
		"provider:\n" +
		"  name: aws-lambda\n" +
		"  runtime: nodejs20.x\n" +
		"functions:\n" +
		"  - name: api\n" +
		"    entry: src/handler.default\n" +
		"workflows:\n" +
		"  - name: wf\n" +
		"    steps:\n" +
		"      - id: s1\n" +
		"        kind: code\n" +
		"        function: legacy-fn\n")
	_, err := LoadFromBytes(content)
	if err == nil {
		t.Fatal("expected unknown field error for workflows.steps.function")
	}
	if !strings.Contains(err.Error(), "field function not found") {
		t.Fatalf("expected unknown function field parse error, got %v", err)
	}
}

func TestLoadFromBytes_RejectsLegacyLayerArnField(t *testing.T) {
	content := []byte("service: svc\n" +
		"provider:\n" +
		"  name: aws-lambda\n" +
		"  runtime: nodejs20.x\n" +
		"functions:\n" +
		"  - name: api\n" +
		"    entry: src/handler.default\n" +
		"layers:\n" +
		"  node-deps:\n" +
		"    arn: arn:aws:lambda:us-east-1:123:layer:node-deps:1\n")
	_, err := LoadFromBytes(content)
	if err == nil {
		t.Fatal("expected unknown field error for layers.<name>.arn")
	}
	if !strings.Contains(err.Error(), "field arn not found") {
		t.Fatalf("expected unknown arn field parse error, got %v", err)
	}
}

func TestLoadFromBytes_RejectsLegacyTopLevelReferenceFields(t *testing.T) {
	content := []byte("service: svc\n" +
		"runtime: nodejs\n" +
		"entry: src/index.ts\n" +
		"providers:\n" +
		"  - aws-lambda\n" +
		"triggers:\n" +
		"  - type: http\n" +
		"    method: GET\n" +
		"    path: /hello\n")
	_, err := LoadFromBytes(content)
	if err == nil {
		t.Fatal("expected unknown field error for legacy top-level reference keys")
	}
	if !strings.Contains(err.Error(), "field runtime not found") {
		t.Fatalf("expected unknown runtime field parse error, got %v", err)
	}
}

package config

import (
	"os"
	"path/filepath"
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

func TestLoad_MinimalLegacy(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "runfabric.yml")
	content := `
service: svc
provider:
  name: aws
  runtime: nodejs20.x
functions:
  api:
    handler: src/handler.default
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Service != "svc" || cfg.Provider.Name != "aws" || cfg.Provider.Runtime != "nodejs20.x" {
		t.Errorf("unexpected config: %+v", cfg)
	}
	if len(cfg.Functions) != 1 || cfg.Functions["api"].Handler != "src/handler.default" {
		t.Errorf("unexpected functions: %+v", cfg.Functions)
	}
}

func TestLoadFromBytes_Minimal(t *testing.T) {
	content := []byte(`
service: svc
provider:
  name: aws
  runtime: nodejs20.x
functions:
  api:
    handler: src/handler.default
`)
	cfg, err := LoadFromBytes(content)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Service != "svc" || cfg.Provider.Name != "aws" || cfg.Provider.Runtime != "nodejs20.x" {
		t.Errorf("unexpected config: %+v", cfg)
	}
	if len(cfg.Functions) != 1 || cfg.Functions["api"].Handler != "src/handler.default" {
		t.Errorf("unexpected functions: %+v", cfg.Functions)
	}
}

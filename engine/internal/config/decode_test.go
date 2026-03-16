package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestFunctionsRaw_UnmarshalYAML_Map(t *testing.T) {
	var cfg Config
	err := yaml.Unmarshal([]byte(`
service: svc
provider:
  name: aws
  runtime: nodejs
functions:
  api:
    handler: src/handler.default
`), &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.FunctionsData == nil || len(cfg.FunctionsData.AsMap) != 1 {
		t.Fatalf("expected AsMap with 1 entry, got %+v", cfg.FunctionsData)
	}
	if cfg.FunctionsData.AsMap["api"].Handler != "src/handler.default" {
		t.Errorf("expected handler src/handler.default, got %q", cfg.FunctionsData.AsMap["api"].Handler)
	}
}

func TestFunctionsRaw_UnmarshalYAML_Array(t *testing.T) {
	var cfg Config
	err := yaml.Unmarshal([]byte(`
service: svc
provider:
  name: aws
  runtime: nodejs
functions:
  - name: api
    entry: src/index.ts
    triggers:
      - type: http
        path: /hello
`), &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.FunctionsData == nil || len(cfg.FunctionsData.AsArray) != 1 {
		t.Fatalf("expected AsArray with 1 entry, got %+v", cfg.FunctionsData)
	}
	if cfg.FunctionsData.AsArray[0].Name != "api" || cfg.FunctionsData.AsArray[0].Entry != "src/index.ts" {
		t.Errorf("unexpected array entry: %+v", cfg.FunctionsData.AsArray[0])
	}
}

func TestFunctionsRaw_UnmarshalYAML_Invalid(t *testing.T) {
	var cfg Config
	err := yaml.Unmarshal([]byte(`
service: svc
provider:
  name: aws
  runtime: nodejs
functions: 123
`), &cfg)
	if err == nil {
		t.Fatal("expected error for scalar functions")
	}
}

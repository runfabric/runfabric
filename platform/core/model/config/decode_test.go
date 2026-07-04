package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestConfig_UnmarshalYAML_FunctionsArray(t *testing.T) {
	var cfg Config
	err := yaml.Unmarshal([]byte(`
service: svc
provider:
  name: aws-lambda
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
	if len(cfg.FunctionsConfig) != 1 {
		t.Fatalf("expected 1 function override, got %+v", cfg.FunctionsConfig)
	}
	if cfg.FunctionsConfig[0].Name != "api" || cfg.FunctionsConfig[0].Entry != "src/index.ts" {
		t.Errorf("unexpected array entry: %+v", cfg.FunctionsConfig[0])
	}
}

func TestConfig_UnmarshalYAML_FunctionsMapAccepted(t *testing.T) {
	// The canonical map form is accepted alongside the reference list form and
	// converts into the same FunctionOverrideConfig shape (docs + examples use it).
	var cfg Config
	err := yaml.Unmarshal([]byte(`
service: svc
provider:
  name: aws-lambda
  runtime: nodejs
functions:
  api:
    handler: src/handler.default
    memory: 256
    environment:
      FOO: bar
`), &cfg)
	if err != nil {
		t.Fatalf("map-form functions must be accepted: %v", err)
	}
	if len(cfg.FunctionsConfig) != 1 {
		t.Fatalf("expected 1 converted function, got %+v", cfg.FunctionsConfig)
	}
	fo := cfg.FunctionsConfig[0]
	if fo.Name != "api" || fo.Entry != "src/handler.default" || fo.Memory != 256 || fo.Environment["FOO"] != "bar" {
		t.Errorf("unexpected converted entry: %+v", fo)
	}
	Normalize(&cfg)
	fn, ok := cfg.Functions["api"]
	if !ok || fn.Handler != "src/handler.default" || fn.Memory != 256 || fn.Environment["FOO"] != "bar" {
		t.Errorf("unexpected normalized function: %+v", cfg.Functions)
	}
}

func TestConfig_UnmarshalYAML_FunctionsMapUnknownKeyRejected(t *testing.T) {
	// Strictness must survive the custom decode: unknown keys still fail.
	var cfg Config
	err := yaml.Unmarshal([]byte(`
service: svc
provider:
  name: aws-lambda
  runtime: nodejs
functions:
  api:
    handler: src/handler.default
    nonsense: true
`), &cfg)
	if err == nil {
		t.Fatal("expected error for unknown function key")
	}
}

func TestConfig_UnmarshalYAML_FunctionsScalarRejected(t *testing.T) {
	var cfg Config
	err := yaml.Unmarshal([]byte(`
service: svc
provider:
  name: aws-lambda
  runtime: nodejs
functions: 123
`), &cfg)
	if err == nil {
		t.Fatal("expected error for scalar functions")
	}
}

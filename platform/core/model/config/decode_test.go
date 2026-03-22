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

func TestConfig_UnmarshalYAML_FunctionsMapRejected(t *testing.T) {
	var cfg Config
	err := yaml.Unmarshal([]byte(`
service: svc
provider:
  name: aws-lambda
  runtime: nodejs
functions:
  api:
    handler: src/handler.default
`), &cfg)
	if err == nil {
		t.Fatal("expected error for map-form functions")
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

package config

import "testing"

// These tests pin the daemon↔runfabric.yml alignment work: full list-format
// parity, the env/environment alias, dual-format functions, and complete
// per-stage function override merging.

func TestListFormatFullFunctionSurface(t *testing.T) {
	cfg, err := LoadFromBytes([]byte(`
service: svc
provider:
  name: aws-lambda
  runtime: nodejs
functions:
  - name: api
    entry: dist/api.handler
    memory: 512
    timeout: 30
    architecture: arm64
    layers: [node-deps]
    tags:
      team: core
    reservedConcurrency: 5
    provisionedConcurrency: 2
    environment:
      FROM_ALIAS: a
      SHARED: alias-loses
    env:
      SHARED: env-wins
    triggers:
      - type: http
        method: get
        path: /api
`))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	fn := cfg.Functions["api"]
	if fn.Memory != 512 || fn.Timeout != 30 || fn.Architecture != "arm64" {
		t.Errorf("scalar fields not mapped: %+v", fn)
	}
	if len(fn.Layers) != 1 || fn.Tags["team"] != "core" || fn.ReservedConcurrency != 5 || fn.ProvisionedConcurrency != 2 {
		t.Errorf("list/map fields not mapped: %+v", fn)
	}
	if fn.Environment["FROM_ALIAS"] != "a" || fn.Environment["SHARED"] != "env-wins" {
		t.Errorf("env alias merge wrong (env must win): %v", fn.Environment)
	}
	if len(fn.Events) != 1 || fn.Events[0].HTTP == nil {
		t.Errorf("triggers not converted to events: %+v", fn.Events)
	}
}

func TestMapFormatLoadsAndResolves(t *testing.T) {
	cfg, err := LoadFromBytes([]byte(`
service: svc
provider:
  name: gcp-functions
  runtime: nodejs20.x
functions:
  handler:
    handler: dist/handler.handler
    memory: 128
    timeout: 10
    events:
      - http:
          path: /hello
          method: get
`))
	if err != nil {
		t.Fatalf("map-format load: %v", err)
	}
	resolved, err := Resolve(cfg, "dev")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	fn, ok := resolved.Functions["handler"]
	if !ok || fn.Handler != "dist/handler.handler" || fn.Memory != 128 || fn.Timeout != 10 {
		t.Fatalf("map-format function not resolved: %+v", resolved.Functions)
	}
	if len(fn.Events) != 1 || fn.Events[0].HTTP == nil || fn.Events[0].HTTP.Path != "/hello" {
		t.Errorf("map-format events lost: %+v", fn.Events)
	}
}

func TestStageOverrideMergesFullFunctionSurface(t *testing.T) {
	cfg, err := LoadFromBytes([]byte(`
service: svc
provider:
  name: aws-lambda
  runtime: nodejs
functions:
  - name: api
    entry: dist/api.handler
    memory: 128
    env:
      BASE: keep
      SHARED: base
stages:
  prod:
    functions:
      api:
        memory: 1024
        timeout: 60
        environment:
          SHARED: prod
          EXTRA: added
        secrets:
          DB_PASSWORD: ${env:PROD_DB_REF,ref}
`))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	resolved, err := Resolve(cfg, "prod")
	if err != nil {
		t.Fatalf("resolve prod: %v", err)
	}
	fn := resolved.Functions["api"]
	if fn.Memory != 1024 || fn.Timeout != 60 {
		t.Errorf("stage memory/timeout not merged: %+v", fn)
	}
	// Per-key env merge: base keys survive, stage keys win/add.
	if fn.Environment["BASE"] != "keep" || fn.Environment["SHARED"] != "prod" || fn.Environment["EXTRA"] != "added" {
		t.Errorf("stage env merge wrong: %v", fn.Environment)
	}
	if fn.Secrets["DB_PASSWORD"] == "" {
		t.Errorf("stage secrets not merged: %v", fn.Secrets)
	}

	// Other stages remain untouched by the prod override.
	dev, err := Resolve(cfg, "dev")
	if err != nil {
		t.Fatalf("resolve dev: %v", err)
	}
	if dev.Functions["api"].Memory != 128 || dev.Functions["api"].Environment["EXTRA"] != "" {
		t.Errorf("dev leaked prod overrides: %+v", dev.Functions["api"])
	}
}

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAiValidate_Disabled(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runfabric.yml")
	cfg := `service: test-ai-validate
provider:
  name: aws-lambda
  runtime: nodejs
functions:
  api:
    handler: index.handler
`
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	root := NewRootCmd()
	root.SetArgs([]string{"ai", "validate", "-c", cfgPath, "--json"})
	root.SetOut(&out)
	root.SetErr(&errOut)
	if err := root.Execute(); err != nil {
		t.Fatalf("ai validate should succeed: %v", err)
	}
	var env map[string]any
	if err := json.NewDecoder(&out).Decode(&env); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if env["ok"] != true {
		t.Errorf("ok = %v", env["ok"])
	}
	if env["command"] != "ai validate" {
		t.Errorf("command = %v", env["command"])
	}
	data, _ := env["result"].(map[string]any)
	if data == nil {
		t.Fatal("result missing")
	}
	if data["enabled"] != false {
		t.Errorf("enabled = %v", data["enabled"])
	}
}

func TestAiValidate_Enabled(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runfabric.yml")
	cfg := `service: test-ai-validate-enabled
provider:
  name: aws-lambda
  runtime: nodejs
functions:
  api:
    handler: index.handler
aiWorkflow:
  enable: true
  entrypoint: start
  nodes:
    - id: start
      type: trigger
      params: {}
    - id: step1
      type: system
      params:
        function: api
  edges:
    - from: start
      to: step1
`
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	root := NewRootCmd()
	root.SetArgs([]string{"ai", "validate", "-c", cfgPath, "--json"})
	root.SetOut(&out)
	root.SetErr(&bytes.Buffer{})
	if err := root.Execute(); err != nil {
		t.Fatalf("ai validate should succeed: %v", err)
	}
	var env map[string]any
	if err := json.NewDecoder(&out).Decode(&env); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if env["ok"] != true {
		t.Errorf("ok = %v", env["ok"])
	}
	data, _ := env["result"].(map[string]any)
	if data == nil {
		t.Fatal("result missing")
	}
	if data["enabled"] != true {
		t.Errorf("enabled = %v", data["enabled"])
	}
	if data["entry"] != "start" {
		t.Errorf("entry = %v", data["entry"])
	}
}

func TestAiGraph_Disabled(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runfabric.yml")
	cfg := `service: test-ai-graph
provider:
  name: aws-lambda
  runtime: nodejs
functions:
  api:
    handler: index.handler
`
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	root := NewRootCmd()
	root.SetArgs([]string{"ai", "graph", "-c", cfgPath, "--json"})
	root.SetOut(&out)
	root.SetErr(&bytes.Buffer{})
	if err := root.Execute(); err != nil {
		t.Fatalf("ai graph with --json writes envelope and returns nil: %v", err)
	}
	var env map[string]any
	if err := json.NewDecoder(&out).Decode(&env); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if env["ok"] != false {
		t.Errorf("ok = %v (expect false when aiWorkflow disabled)", env["ok"])
	}
	errObj, _ := env["error"].(map[string]any)
	if errObj != nil {
		if errObj["code"] != "ai_workflow_disabled" {
			t.Errorf("error.code = %v", errObj["code"])
		}
	}
}

func TestAiGraph_Enabled(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runfabric.yml")
	cfg := `service: test-ai-graph-enabled
provider:
  name: aws-lambda
  runtime: nodejs
functions:
  api:
    handler: index.handler
aiWorkflow:
  enable: true
  entrypoint: start
  nodes:
    - id: start
      type: trigger
      params: {}
    - id: step1
      type: system
      params:
        function: api
  edges:
    - from: start
      to: step1
`
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	root := NewRootCmd()
	root.SetArgs([]string{"ai", "graph", "-c", cfgPath, "--json"})
	root.SetOut(&out)
	root.SetErr(&bytes.Buffer{})
	if err := root.Execute(); err != nil {
		t.Fatalf("ai graph should succeed: %v", err)
	}
	var env map[string]any
	if err := json.NewDecoder(&out).Decode(&env); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if env["ok"] != true {
		t.Errorf("ok = %v", env["ok"])
	}
	data, _ := env["result"].(map[string]any)
	if data == nil {
		t.Fatal("result missing")
	}
	if data["entrypoint"] != "start" {
		t.Errorf("entrypoint = %v", data["entrypoint"])
	}
	if data["hash"] == nil || data["hash"] == "" {
		t.Errorf("hash missing or empty")
	}
	order, _ := data["order"].([]any)
	if len(order) != 2 {
		t.Errorf("order length = %d", len(order))
	}
}

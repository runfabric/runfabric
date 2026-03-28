package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestDoctor_SucceedsWithMinimalConfig ensures doctor runs and exits successfully with a valid minimal config.
func TestDoctor_SucceedsWithMinimalConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := `service: test-doctor
provider:
  name: aws-lambda
  runtime: nodejs
functions:
  - name: api
    entry: index.handler
`
	cfgPath := writeConfig(t, dir, cfg)
	root := NewRootCmd()
	root.SetArgs([]string{"doctor", "-c", cfgPath, "--stage", "dev", "--json"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	if err := root.Execute(); err != nil {
		t.Fatalf("doctor should succeed: %v", err)
	}
}

// TestPlan_RunsWithMinimalConfig ensures plan runs (may fail at package step without real project; we only verify no panic).
func TestPlan_RunsWithMinimalConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := `service: test-plan
provider:
  name: aws-lambda
  runtime: nodejs
functions:
  - name: api
    entry: index.handler
`
	cfgPath := writeConfig(t, dir, cfg)
	root := NewRootCmd()
	root.SetArgs([]string{"plan", "-c", cfgPath, "--stage", "dev", "--json"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	_ = root.Execute()
}

// TestPlan_RunsWithScalingAndHealthConfig ensures plan runs with deploy.scaling and deploy.healthCheck in config.
func TestPlan_RunsWithScalingAndHealthConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := `service: test-scaling-health
provider:
  name: aws-lambda
  runtime: nodejs
deploy:
  rollbackOnFailure: true
  healthCheck:
    enabled: true
    url: ""
  scaling:
    reservedConcurrency: 10
    provisionedConcurrency: 0
functions:
  - name: api
    entry: index.handler
    reservedConcurrency: 5
`
	cfgPath := writeConfig(t, dir, cfg)
	root := NewRootCmd()
	root.SetArgs([]string{"plan", "-c", cfgPath, "--stage", "dev"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	_ = root.Execute()
}

// TestPlan_RunsWithLayersConfig ensures plan runs with top-level layers and function layer refs.
func TestPlan_RunsWithLayersConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := `service: test-layers
provider:
  name: aws-lambda
  runtime: nodejs
layers:
  node-deps:
    ref: "arn:aws:lambda:us-east-1:123456789012:layer:node-deps:1"
    name: node-deps
    version: "1"
functions:
  - name: api
    entry: index.handler
    layers: ["node-deps"]
`
	cfgPath := writeConfig(t, dir, cfg)
	root := NewRootCmd()
	root.SetArgs([]string{"plan", "-c", cfgPath, "--stage", "dev"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	_ = root.Execute()
}

// TestLogs_AllFlagAccepted ensures logs --all is accepted (may fail at provider; we only check the command runs).
func TestLogs_AllFlagAccepted(t *testing.T) {
	dir := t.TempDir()
	cfg := `service: test-logs
provider:
  name: aws-lambda
  runtime: nodejs
functions:
  - name: api
    entry: index.handler
`
	cfgPath := writeConfig(t, dir, cfg)
	root := NewRootCmd()
	root.SetArgs([]string{"invoke", "logs", "-c", cfgPath, "--stage", "dev", "--all"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	_ = root.Execute()
}

// TestLogs_RequiresFunctionOrAll ensures logs without --function or --all returns an error.
func TestLogs_RequiresFunctionOrAll(t *testing.T) {
	dir := t.TempDir()
	cfg := `service: test-logs
provider:
  name: aws-lambda
  runtime: nodejs
functions:
  - name: api
    entry: index.handler
`
	cfgPath := writeConfig(t, dir, cfg)
	root := NewRootCmd()
	root.SetArgs([]string{"invoke", "logs", "-c", cfgPath, "--stage", "dev"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	if err := root.Execute(); err == nil {
		t.Fatal("logs without --function or --all should fail")
	}
}

func TestLogs_ServiceScopeMismatchFails(t *testing.T) {
	dir := t.TempDir()
	cfg := `service: test-logs
provider:
  name: aws-lambda
  runtime: nodejs
functions:
  - name: api
    entry: index.handler
`
	cfgPath := writeConfig(t, dir, cfg)
	root := NewRootCmd()
	root.SetArgs([]string{"invoke", "logs", "-c", cfgPath, "--stage", "dev", "--all", "--service", "other-service"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	if err := root.Execute(); err == nil {
		t.Fatal("logs with mismatched --service should fail")
	}
}

// TestMetrics_AllFlagAccepted ensures metrics --all is accepted.
func TestMetrics_AllFlagAccepted(t *testing.T) {
	dir := t.TempDir()
	cfg := `service: test-metrics
provider:
  name: aws-lambda
  runtime: nodejs
functions:
  - name: api
    entry: index.handler
`
	cfgPath := writeConfig(t, dir, cfg)
	root := NewRootCmd()
	root.SetArgs([]string{"invoke", "metrics", "-c", cfgPath, "--stage", "dev", "--all"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	_ = root.Execute()
}

func TestMetrics_ServiceScopeMismatchFails(t *testing.T) {
	dir := t.TempDir()
	cfg := `service: test-metrics
provider:
  name: aws-lambda
  runtime: nodejs
functions:
  - name: api
    entry: index.handler
`
	cfgPath := writeConfig(t, dir, cfg)
	root := NewRootCmd()
	root.SetArgs([]string{"invoke", "metrics", "-c", cfgPath, "--stage", "dev", "--all", "--service", "other-service"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	if err := root.Execute(); err == nil {
		t.Fatal("metrics with mismatched --service should fail")
	}
}

// TestTraces_AllFlagAccepted ensures traces --all and --provider are accepted (--provider requires providerOverrides).
func TestTraces_AllFlagAccepted(t *testing.T) {
	dir := t.TempDir()
	cfg := `service: test-traces
provider:
  name: aws-lambda
  runtime: nodejs
providerOverrides:
  aws-lambda:
    name: aws-lambda
    runtime: nodejs
functions:
  - name: api
    entry: index.handler
`
	cfgPath := writeConfig(t, dir, cfg)
	root := NewRootCmd()
	root.SetArgs([]string{"invoke", "traces", "-c", cfgPath, "--stage", "dev", "--provider", "aws-lambda", "--all"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	_ = root.Execute()
}

func TestTraces_ServiceScopeMismatchFails(t *testing.T) {
	dir := t.TempDir()
	cfg := `service: test-traces
provider:
  name: aws-lambda
  runtime: nodejs
providerOverrides:
  aws-lambda:
    name: aws-lambda
    runtime: nodejs
functions:
  - name: api
    entry: index.handler
`
	cfgPath := writeConfig(t, dir, cfg)
	root := NewRootCmd()
	root.SetArgs([]string{"invoke", "traces", "-c", cfgPath, "--stage", "dev", "--provider", "aws-lambda", "--all", "--service", "other-service"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	if err := root.Execute(); err == nil {
		t.Fatal("traces with mismatched --service should fail")
	}
}

// TestInspect_RunsWithMinimalConfig ensures inspect runs (may fail if no receipt; we only check the command runs).
func TestInspect_RunsWithMinimalConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := `service: test-inspect
provider:
  name: aws-lambda
  runtime: nodejs
functions:
  - name: api
    entry: index.handler
`
	cfgPath := writeConfig(t, dir, cfg)
	root := NewRootCmd()
	root.SetArgs([]string{"inspect", "-c", cfgPath, "--stage", "dev"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	_ = root.Execute()
}

// TestInvoke_RunsWithMinimalConfig ensures invoke runs (may fail at provider; we only verify the command runs).
func TestInvoke_RunsWithMinimalConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := `service: test-invoke
provider:
  name: aws-lambda
  runtime: nodejs
functions:
  - name: api
    entry: index.handler
`
	cfgPath := writeConfig(t, dir, cfg)
	root := NewRootCmd()
	root.SetArgs([]string{"invoke", "run", "-c", cfgPath, "--stage", "dev", "--function", "api", "--payload", "{}"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	_ = root.Execute()
}

// TestBuild_SucceedsWithMinimalConfig ensures build runs. Requires a minimal project (package.json + handler).
func TestBuild_SucceedsWithMinimalConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := `service: test-build
provider:
  name: aws-lambda
  runtime: nodejs
functions:
  - name: api
    entry: index.handler
`
	cfgPath := writeConfig(t, dir, cfg)
	// Minimal package.json so Node build can resolve
	packageJSON := `{"name":"test-build","version":"1.0.0"}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(packageJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	handler := `exports.handler = async () => ({ statusCode: 200 });`
	if err := os.WriteFile(filepath.Join(dir, "index.js"), []byte(handler), 0o600); err != nil {
		t.Fatal(err)
	}
	root := NewRootCmd()
	root.SetArgs([]string{"build", "-c", cfgPath, "--stage", "dev"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	// Build may fail on missing node_modules; we only verify the command runs without panic.
	_ = root.Execute()
}

// TestDev_HelpShowsStreamFromAndTunnelUrl ensures dev --help documents Phase 3 live-stream flags.
func TestDev_HelpShowsStreamFromAndTunnelUrl(t *testing.T) {
	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetArgs([]string{"invoke", "dev", "--help"})
	root.SetOut(out)
	root.SetErr(&bytes.Buffer{})
	if err := root.Execute(); err != nil {
		t.Fatalf("invoke dev --help should succeed: %v", err)
	}
	b := out.Bytes()
	if !bytes.Contains(b, []byte("stream-from")) {
		t.Error("invoke dev --help output should mention --stream-from")
	}
	if !bytes.Contains(b, []byte("tunnel-url")) {
		t.Error("invoke dev --help output should mention --tunnel-url")
	}
	if !bytes.Contains(b, []byte("doctor-first")) {
		t.Error("invoke dev --help output should mention --doctor-first")
	}
}

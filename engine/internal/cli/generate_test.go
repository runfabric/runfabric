package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateFunction_RequiresName(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runfabric.yml")
	writeMinimalConfig(t, cfgPath)

	opts := &GlobalOptions{ConfigPath: cfgPath}
	cmd := newGenerateFunctionCmd(opts)
	cmd.SetArgs([]string{"--trigger", "http"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("generate function without name should fail")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("expected 'name is required' in error, got: %v", err)
	}
}

func TestGenerateFunction_UnsupportedTrigger(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runfabric.yml")
	writeMinimalConfig(t, cfgPath)

	opts := &GlobalOptions{ConfigPath: cfgPath}
	cmd := newGenerateFunctionCmd(opts)
	cmd.SetArgs([]string{"hello", "--trigger", "kafka"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("generate function --trigger kafka should fail (MVP: http|cron|queue only)")
	}
	if !strings.Contains(err.Error(), "trigger must be http, cron, or queue") {
		t.Errorf("expected trigger validation error, got: %v", err)
	}
}

func TestGenerateFunction_ProviderDoesNotSupportTrigger(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runfabric.yml")
	// fly-machines does not support queue per capability matrix
	writeGenerateConfig(t, cfgPath, `service: test
provider:
  name: fly-machines
  runtime: nodejs20.x
functions:
  handler:
    handler: src/handler.handler
    memory: 128
`)

	opts := &GlobalOptions{ConfigPath: cfgPath}
	cmd := newGenerateFunctionCmd(opts)
	cmd.SetArgs([]string{"worker", "--trigger", "queue", "--no-backup"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("generate function with provider that does not support trigger should fail")
	}
	if !strings.Contains(err.Error(), "does not support trigger") {
		t.Errorf("expected provider/trigger matrix error, got: %v", err)
	}
}

func TestGenerateFunction_Success(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runfabric.yml")
	writeMinimalConfig(t, cfgPath)

	opts := &GlobalOptions{ConfigPath: cfgPath}
	cmd := newGenerateFunctionCmd(opts)
	cmd.SetArgs([]string{"hello", "--trigger", "http", "--route", "GET:/hello", "--no-backup"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("generate function should succeed: %v", err)
	}

	handlerPath := filepath.Join(dir, "src", "hello.js")
	if _, err := os.Stat(handlerPath); err != nil {
		t.Fatalf("handler file should exist at %s: %v", handlerPath, err)
	}
	data, _ := os.ReadFile(handlerPath)
	if !strings.Contains(string(data), "exports.handler") {
		t.Errorf("handler should contain exports.handler, got: %s", string(data))
	}

	cfgData, _ := os.ReadFile(cfgPath)
	if !strings.Contains(string(cfgData), "hello:") {
		t.Errorf("runfabric.yml should contain function hello, got: %s", string(cfgData))
	}
	if !strings.Contains(string(cfgData), "src/hello.js") {
		t.Errorf("runfabric.yml should contain handler path src/hello.js, got: %s", string(cfgData))
	}
}

func TestGenerateFunction_Collision(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runfabric.yml")
	writeMinimalConfig(t, cfgPath)

	opts := &GlobalOptions{ConfigPath: cfgPath}
	cmd := newGenerateFunctionCmd(opts)
	cmd.SetArgs([]string{"handler", "--trigger", "http", "--no-backup"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("generate function with existing name should fail")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' in error, got: %v", err)
	}
}

func TestGenerateFunction_DryRun(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runfabric.yml")
	writeMinimalConfig(t, cfgPath)

	opts := &GlobalOptions{ConfigPath: cfgPath}
	cmd := newGenerateFunctionCmd(opts)
	cmd.SetArgs([]string{"dryrun-fn", "--trigger", "cron", "--schedule", "rate(10 minutes)", "--dry-run"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("generate function --dry-run should succeed: %v", err)
	}

	// Dry-run must not create any files
	if _, err := os.Stat(filepath.Join(dir, "src", "dryrun-fn.js")); err == nil {
		t.Error("dry-run should not create handler file")
	}
	// Config should be unchanged (still only "handler" function)
	cfgData, _ := os.ReadFile(cfgPath)
	if strings.Count(string(cfgData), "dryrun-fn") > 0 {
		t.Error("dry-run should not patch runfabric.yml")
	}
}

// TestInitThenGenerateFunction runs init to create a project, then generate function to add a second function (integration).
func TestInitThenGenerateFunction(t *testing.T) {
	dir := t.TempDir()

	// 1. Init project
	initOpts := &GlobalOptions{}
	initCmd := newInitCmd(initOpts)
	initCmd.SetArgs([]string{
		"--dir", dir,
		"--no-interactive",
		"--provider", "aws-lambda",
		"--template", "http",
		"--lang", "js",
		"--service", "init-gen-test",
	})
	initCmd.SetOut(&bytes.Buffer{})
	initCmd.SetErr(&bytes.Buffer{})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("init should succeed: %v", err)
	}

	cfgPath := filepath.Join(dir, "runfabric.yml")
	if _, err := os.Stat(cfgPath); err != nil {
		t.Fatalf("init should create runfabric.yml: %v", err)
	}

	// 2. Generate a new function
	genOpts := &GlobalOptions{ConfigPath: cfgPath}
	genCmd := newGenerateFunctionCmd(genOpts)
	genCmd.SetArgs([]string{"second", "--trigger", "http", "--route", "POST:/second", "--no-backup"})
	genCmd.SetOut(&bytes.Buffer{})
	genCmd.SetErr(&bytes.Buffer{})
	if err := genCmd.Execute(); err != nil {
		t.Fatalf("generate function after init should succeed: %v", err)
	}

	// 3. Assert new handler and config entry
	handlerPath := filepath.Join(dir, "src", "second.js")
	if _, err := os.Stat(handlerPath); err != nil {
		t.Fatalf("generate should create src/second.js: %v", err)
	}
	cfgData, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(cfgData)
	if !strings.Contains(content, "second:") {
		t.Errorf("config should contain function 'second', got: %s", content)
	}
	if !strings.Contains(content, "src/second.js") {
		t.Errorf("config should contain handler src/second.js, got: %s", content)
	}
	if !strings.Contains(content, "post") || !strings.Contains(content, "/second") {
		t.Errorf("config should contain route POST /second, got: %s", content)
	}
}

func writeMinimalConfig(t *testing.T, path string) {
	t.Helper()
	writeGenerateConfig(t, path, `service: test
provider:
  name: aws-lambda
  runtime: nodejs20.x
functions:
  handler:
    handler: src/handler.handler
    memory: 128
`)
}

func writeGenerateConfig(t *testing.T, path, yaml string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
}

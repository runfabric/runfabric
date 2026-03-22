package project

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withStdinInput(t *testing.T, input string, fn func()) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdin
	os.Stdin = r
	defer func() {
		os.Stdin = old
		_ = r.Close()
	}()
	if _, err := w.WriteString(input); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	fn()
}

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

func TestGenerateFunction_InteractiveFlagConflict(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runfabric.yml")
	writeMinimalConfig(t, cfgPath)

	opts := &GlobalOptions{ConfigPath: cfgPath}
	cmd := newGenerateFunctionCmd(opts)
	cmd.SetArgs([]string{"hello", "--interactive", "--no-interactive"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected generate function with --interactive and --no-interactive to fail")
	}
	if !strings.Contains(err.Error(), "cannot be used together") {
		t.Errorf("expected conflict error, got: %v", err)
	}
}

func TestGenerateResource_InteractiveFlagConflict(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runfabric.yml")
	writeMinimalConfig(t, cfgPath)

	opts := &GlobalOptions{ConfigPath: cfgPath}
	cmd := newGenerateResourceCmd(opts)
	cmd.SetArgs([]string{"db", "--interactive", "--no-interactive"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected generate resource with --interactive and --no-interactive to fail")
	}
	if !strings.Contains(err.Error(), "cannot be used together") {
		t.Errorf("expected conflict error, got: %v", err)
	}
}

func TestGenerateAddon_InteractiveFlagConflict(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runfabric.yml")
	writeMinimalConfig(t, cfgPath)

	opts := &GlobalOptions{ConfigPath: cfgPath}
	cmd := newGenerateAddonCmd(opts)
	cmd.SetArgs([]string{"sentry", "--interactive", "--no-interactive"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected generate addon with --interactive and --no-interactive to fail")
	}
	if !strings.Contains(err.Error(), "cannot be used together") {
		t.Errorf("expected conflict error, got: %v", err)
	}
}

func TestGenerateProviderOverride_InteractiveFlagConflict(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runfabric.yml")
	writeMinimalConfig(t, cfgPath)

	opts := &GlobalOptions{ConfigPath: cfgPath}
	cmd := newGenerateProviderOverrideCmd(opts)
	cmd.SetArgs([]string{"aws", "--interactive", "--no-interactive"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected generate provider-override with --interactive and --no-interactive to fail")
	}
	if !strings.Contains(err.Error(), "cannot be used together") {
		t.Errorf("expected conflict error, got: %v", err)
	}
}

func TestGenerateProviderOverride_ProviderDoesNotSupportExistingTriggers(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runfabric.yml")
	writeGenerateConfig(t, cfgPath, "service: test\n"+
		"provider:\n"+
		"  name: aws-lambda\n"+
		"  runtime: nodejs20.x\n"+
		"functions:\n"+
		"  - name: worker\n"+
		"    entry: src/worker.handler\n"+
		"    triggers:\n"+
		"      - type: queue\n"+
		"        queue: jobs\n")

	opts := &GlobalOptions{ConfigPath: cfgPath}
	cmd := newGenerateProviderOverrideCmd(opts)
	cmd.SetArgs([]string{"fly", "--provider", "fly-machines", "--runtime", "nodejs20.x", "--region", "iad", "--no-backup"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected provider override to fail for unsupported existing triggers")
	}
	if !strings.Contains(err.Error(), "does not support existing project triggers") {
		t.Fatalf("expected trigger capability error, got %v", err)
	}
}

func TestGenerateProviderOverride_InteractivePromptRePromptsUnsupportedProvider(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runfabric.yml")
	writeGenerateConfig(t, cfgPath, "service: test\n"+
		"provider:\n"+
		"  name: aws-lambda\n"+
		"  runtime: nodejs20.x\n"+
		"functions:\n"+
		"  - name: worker\n"+
		"    entry: src/worker.handler\n"+
		"    triggers:\n"+
		"      - type: queue\n"+
		"        queue: jobs\n")

	opts := &GlobalOptions{ConfigPath: cfgPath}
	cmd := newGenerateProviderOverrideCmd(opts)
	cmd.SetArgs([]string{"--interactive", "--no-backup"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	withStdinInput(t, strings.Join([]string{
		"fly",          // key
		"fly-machines", // unsupported provider for queue trigger
		"aws-lambda",   // supported retry
		"nodejs20.x",   // runtime
		"us-east-1",    // region
		"y",            // confirm
	}, "\n")+"\n", func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("interactive generate provider-override should succeed: %v", err)
		}
	})

	cfgData, _ := os.ReadFile(cfgPath)
	content := string(cfgData)
	if !strings.Contains(content, "providerOverrides:") || !strings.Contains(content, "fly:") || !strings.Contains(content, "name: aws-lambda") {
		t.Fatalf("expected compatible provider override in config, got: %s", content)
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
  - name: handler
    entry: src/handler.handler
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
	if !strings.Contains(string(cfgData), "name: hello") {
		t.Errorf("runfabric.yml should contain function hello, got: %s", string(cfgData))
	}
	if !strings.Contains(string(cfgData), "entry: src/hello.js") {
		t.Errorf("runfabric.yml should contain entry path src/hello.js, got: %s", string(cfgData))
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

func TestGenerateFunction_InteractivePromptFlow(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runfabric.yml")
	writeMinimalConfig(t, cfgPath)

	opts := &GlobalOptions{ConfigPath: cfgPath}
	cmd := newGenerateFunctionCmd(opts)
	cmd.SetArgs([]string{"--interactive", "--no-backup"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	withStdinInput(t, strings.Join([]string{
		"prompted-fn",     // function name
		"js",              // language
		"http",            // trigger
		"src/prompted.js", // entry
		"GET:/prompted",   // route
		"y",               // confirm
	}, "\n")+"\n", func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("interactive generate function should succeed: %v", err)
		}
	})

	cfgData, _ := os.ReadFile(cfgPath)
	content := string(cfgData)
	if !strings.Contains(content, "name: prompted-fn") {
		t.Fatalf("expected prompted function in config, got: %s", content)
	}
	entryPath := "src/prompted-fn.js"
	if !strings.Contains(content, "entry: "+entryPath) {
		t.Fatalf("expected prompted function entry %s in config, got: %s", entryPath, content)
	}
	if _, err := os.Stat(filepath.Join(dir, entryPath)); err != nil {
		t.Fatalf("expected prompted handler file %s: %v", entryPath, err)
	}
}

func TestGenerateResource_InteractivePromptFlow(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runfabric.yml")
	writeMinimalConfig(t, cfgPath)

	opts := &GlobalOptions{ConfigPath: cfgPath}
	cmd := newGenerateResourceCmd(opts)
	cmd.SetArgs([]string{"--interactive", "--no-backup"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	withStdinInput(t, strings.Join([]string{
		"db2",          // name
		"database",     // type
		"DATABASE_URL", // connection env
		"y",            // confirm
	}, "\n")+"\n", func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("interactive generate resource should succeed: %v", err)
		}
	})

	cfgData, _ := os.ReadFile(cfgPath)
	if !strings.Contains(string(cfgData), "db2:") {
		t.Fatalf("expected resource in config, got: %s", string(cfgData))
	}
}

func TestGenerateAddon_InteractivePromptFlow(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runfabric.yml")
	writeMinimalConfig(t, cfgPath)

	opts := &GlobalOptions{ConfigPath: cfgPath}
	cmd := newGenerateAddonCmd(opts)
	cmd.SetArgs([]string{"--interactive", "--no-backup"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	withStdinInput(t, strings.Join([]string{
		"sentry2", // addon name
		"",        // version optional
		"y",       // confirm
	}, "\n")+"\n", func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("interactive generate addon should succeed: %v", err)
		}
	})

	cfgData, _ := os.ReadFile(cfgPath)
	if !strings.Contains(string(cfgData), "sentry2:") {
		t.Fatalf("expected addon in config, got: %s", string(cfgData))
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
	if !strings.Contains(content, "name: second") {
		t.Errorf("config should contain function 'second', got: %s", content)
	}
	if !strings.Contains(content, "entry: src/second.js") {
		t.Errorf("config should contain entry src/second.js, got: %s", content)
	}
	if !strings.Contains(content, "POST") || !strings.Contains(content, "/second") {
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
  - name: handler
    entry: src/handler.handler
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

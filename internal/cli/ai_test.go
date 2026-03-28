package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAiCommand_IsRemovedInFavorOfWorkflow(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runfabric.yml")
	cfg := `service: test-workflow-cli
provider:
  name: aws-lambda
  runtime: nodejs
functions:
  - name: api
    entry: index.handler
workflows:
  - name: hello-flow
    steps:
      - id: api-step
        kind: code
`
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	root := NewRootCmd()
	root.SetArgs([]string{"ai", "validate", "-c", cfgPath, "--json"})
	root.SetOut(&out)
	root.SetErr(&bytes.Buffer{})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected ai command to be unavailable")
	}
	if !strings.Contains(err.Error(), "unknown command \"ai\"") {
		t.Fatalf("expected unknown ai command error, got: %v", err)
	}
}

func TestWorkflowCommand_RemainsAvailable(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runfabric.yml")
	cfg := `service: test-workflow-available
provider:
  name: aws-lambda
  runtime: nodejs
functions:
  - name: api
    entry: index.handler
workflows:
  - name: hello-flow
    steps:
      - id: api-step
        kind: code
`
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	root := NewRootCmd()
	root.SetArgs([]string{"workflow", "run", "-c", cfgPath, "--name", "hello-flow", "--json"})
	root.SetOut(&out)
	root.SetErr(&bytes.Buffer{})
	if err := root.Execute(); err != nil {
		t.Fatalf("workflow run should succeed: %v", err)
	}
}

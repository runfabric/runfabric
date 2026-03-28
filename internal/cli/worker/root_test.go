package worker

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestWorkerRootCmd_Surface(t *testing.T) {
	root := NewRootCmd()
	if root.Use != "runfabricw" {
		t.Fatalf("expected worker root use runfabricw, got %q", root.Use)
	}

	workflowCmd := findTopLevelCommand(root, "workflow")
	if workflowCmd == nil || workflowCmd.Hidden {
		t.Fatal("workflow command should be visible in worker root")
	}

	for _, name := range []string{"deploy", "state", "router"} {
		cmd := findTopLevelCommand(root, name)
		if cmd == nil {
			t.Fatalf("expected guard command for %q", name)
		}
		if !cmd.Hidden {
			t.Fatalf("guard command %q should be hidden", name)
		}
	}

	routerCmd := findTopLevelCommand(root, "router")
	if routerCmd == nil {
		t.Fatal("expected router guard command to exist")
	}
}

func TestWorkerRootCmd_RejectsControlPlaneCommands(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "deploy", args: []string{"deploy"}},
		{name: "state list", args: []string{"state", "list"}},
		{name: "router status", args: []string{"router", "status"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := NewRootCmd()
			root.SetOut(&bytes.Buffer{})
			root.SetErr(&bytes.Buffer{})
			root.SetArgs(tc.args)

			err := root.Execute()
			if err == nil {
				t.Fatalf("expected %q to fail in worker root", tc.name)
			}
			if !strings.Contains(err.Error(), "not available in runfabricw") {
				t.Fatalf("expected actionable runfabricw error, got: %v", err)
			}
			if !strings.Contains(err.Error(), "Use runfabric") {
				t.Fatalf("expected migration hint to use runfabric, got: %v", err)
			}
		})
	}
}

func TestWorkerRootCmd_WorkflowRemainsAvailable(t *testing.T) {
	dir := t.TempDir()
	cfg := `service: worker-root-workflow
provider:
  name: aws-lambda
  runtime: nodejs
functions:
  - name: api
    entry: index.handler
workflows:
  - name: hello-flow
    steps:
      - id: s1
        kind: code
      - id: s2
        kind: code
`
	cfgPath := writeConfig(t, dir, cfg)

	root := NewRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"workflow", "run", "-c", cfgPath, "--name", "hello-flow", "--json", "--input", `{"ticket":"A-1"}`})
	if err := root.Execute(); err != nil {
		t.Fatalf("workflow run should succeed in worker root: %v", err)
	}
}

func writeConfig(t *testing.T, dir, content string) string {
	t.Helper()
	cfgPath := filepath.Join(dir, "runfabric.yml")
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return cfgPath
}

func findTopLevelCommand(root *cobra.Command, name string) *cobra.Command {
	for _, cmd := range root.Commands() {
		if cmd.Name() == name || cmd.HasAlias(name) {
			return cmd
		}
	}
	return nil
}

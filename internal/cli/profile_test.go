package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestNewRootCmd_ControlPlaneSurface(t *testing.T) {
	root := NewRootCmd()
	if root.Use != "runfabric" {
		t.Fatalf("expected default root use runfabric, got %q", root.Use)
	}

	deployCmd := findTopLevelCommand(root, "deploy")
	if deployCmd == nil {
		t.Fatal("expected deploy command in CLI profile")
	}
	if deployCmd.Hidden {
		t.Fatal("deploy command should not be hidden in CLI profile")
	}

	workflowCmd := findTopLevelCommand(root, "workflow")
	if workflowCmd == nil || workflowCmd.Hidden {
		t.Fatal("workflow command should be available in CLI profile")
	}
}

func findTopLevelCommand(root *cobra.Command, name string) *cobra.Command {
	for _, cmd := range root.Commands() {
		if cmd.Name() == name || cmd.HasAlias(name) {
			return cmd
		}
	}
	return nil
}

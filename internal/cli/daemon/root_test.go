package daemon

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestDaemonRootCmd_Surface(t *testing.T) {
	root := NewRootCmd()
	if root.Use != "runfabricd" {
		t.Fatalf("expected daemon root use runfabricd, got %q", root.Use)
	}

	for _, name := range []string{"start", "stop", "restart", "status"} {
		if findTopLevelCommand(root, name) == nil {
			t.Fatalf("expected %q command in daemon root", name)
		}
	}
	if findTopLevelCommand(root, "daemon") != nil {
		t.Fatal("daemon subcommand should not be present in runfabricd root")
	}

	if findTopLevelCommand(root, "deploy") != nil {
		t.Fatal("deploy should not be present in daemon root")
	}
}

func TestDaemonRootCmd_RejectsControlPlaneCommands(t *testing.T) {
	root := NewRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"deploy"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected deploy to be unavailable in daemon root")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("expected unknown command error, got: %v", err)
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

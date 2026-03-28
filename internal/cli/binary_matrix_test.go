package cli

import (
	"bytes"
	"strings"
	"testing"

	daemoncli "github.com/runfabric/runfabric/internal/cli/daemon"
	workercli "github.com/runfabric/runfabric/internal/cli/worker"
)

func TestBinaryCommandSurfaceMatrix(t *testing.T) {
	control := NewRootCmd()
	if findTopLevelCommand(control, "deploy") == nil {
		t.Fatal("runfabric should expose deploy")
	}
	if findTopLevelCommand(control, "workflow") == nil {
		t.Fatal("runfabric should expose workflow")
	}
	if findTopLevelCommand(control, "daemon") != nil {
		t.Fatal("runfabric should not expose daemon command")
	}

	daemon := daemoncli.NewRootCmd()
	if findTopLevelCommand(daemon, "start") == nil {
		t.Fatal("runfabricd should expose start")
	}
	if findTopLevelCommand(daemon, "deploy") != nil {
		t.Fatal("runfabricd should not expose deploy")
	}
	if findTopLevelCommand(daemon, "workflow") != nil {
		t.Fatal("runfabricd should not expose workflow")
	}

	worker := workercli.NewRootCmd()
	if workflow := findTopLevelCommand(worker, "workflow"); workflow == nil || workflow.Hidden {
		t.Fatal("runfabricw should expose workflow")
	}
	if deploy := findTopLevelCommand(worker, "deploy"); deploy == nil || !deploy.Hidden {
		t.Fatal("runfabricw deploy should be hidden guard")
	}
	if start := findTopLevelCommand(worker, "start"); start != nil {
		t.Fatal("runfabricw should not expose daemon start")
	}
}

func TestBinaryCrossCommandMisuseErrors(t *testing.T) {
	t.Run("runfabric daemon unknown", func(t *testing.T) {
		root := NewRootCmd()
		root.SetOut(&bytes.Buffer{})
		root.SetErr(&bytes.Buffer{})
		root.SetArgs([]string{"daemon", "status"})
		err := root.Execute()
		if err == nil {
			t.Fatal("expected runfabric daemon status to fail")
		}
		if !strings.Contains(err.Error(), "unknown command") {
			t.Fatalf("expected unknown command message, got: %v", err)
		}
	})

	t.Run("runfabricd deploy unknown", func(t *testing.T) {
		root := daemoncli.NewRootCmd()
		root.SetOut(&bytes.Buffer{})
		root.SetErr(&bytes.Buffer{})
		root.SetArgs([]string{"deploy"})
		err := root.Execute()
		if err == nil {
			t.Fatal("expected runfabricd deploy to fail")
		}
		if !strings.Contains(err.Error(), "unknown command") {
			t.Fatalf("expected unknown command error, got: %v", err)
		}
	})

	t.Run("runfabricw deploy guarded", func(t *testing.T) {
		root := workercli.NewRootCmd()
		root.SetOut(&bytes.Buffer{})
		root.SetErr(&bytes.Buffer{})
		root.SetArgs([]string{"deploy"})
		err := root.Execute()
		if err == nil {
			t.Fatal("expected runfabricw deploy to fail")
		}
		if !strings.Contains(err.Error(), "not available in runfabricw") {
			t.Fatalf("expected worker guard message, got: %v", err)
		}
	})
}

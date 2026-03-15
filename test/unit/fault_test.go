package unit

import (
	"testing"

	"github.com/runfabric/runfabric/internal/deployexec"
)

func TestFaultConfig(t *testing.T) {
	f := deployexec.FaultConfig{
		Enabled:         true,
		FailBeforePhase: "discover_state",
		FailAfterPhase:  "ensure_routes",
		FailOnResource:  "lambda:hello",
	}

	if err := f.CheckBefore("discover_state"); err == nil {
		t.Fatal("expected before-phase fault")
	}

	if err := f.CheckAfter("ensure_routes"); err == nil {
		t.Fatal("expected after-phase fault")
	}

	if err := f.CheckResource("lambda:hello"); err == nil {
		t.Fatal("expected resource fault")
	}

	if err := f.CheckBefore("package_artifacts"); err != nil {
		t.Fatalf("unexpected fault: %v", err)
	}
}

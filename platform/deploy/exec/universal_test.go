package exec

import (
	"context"
	"testing"

	providers "github.com/runfabric/runfabric/platform/core/contracts/provider"
	"github.com/runfabric/runfabric/platform/core/model/config"
	"github.com/runfabric/runfabric/platform/core/state/transactions"
)

// TestRunDeploy_FreshDeployReturnsResult covers the normal path: the deploy
// phase runs and its result is returned.
func TestRunDeploy_FreshDeployReturnsResult(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{Service: "svc"}
	journal := transactions.NewJournal("svc", "dev", "deploy", transactions.NewFileBackend(root))

	calls := 0
	res, err := RunDeploy(context.Background(), cfg, "dev", root, FaultConfig{}, journal, func(context.Context) (*providers.DeployResult, error) {
		calls++
		return &providers.DeployResult{Provider: "test"}, nil
	})
	if err != nil {
		t.Fatalf("RunDeploy: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil result")
	}
	if calls != 1 {
		t.Errorf("deployFn calls = %d, want 1", calls)
	}
}

// TestRunDeploy_ResumeWithCompletedPhaseStillReturnsResult guards the regression
// where a journal resume skips the already-"done" deploy phase, leaving the
// result nil and panicking the caller. The provider deploy must be re-run so a
// non-nil result is returned.
func TestRunDeploy_ResumeWithCompletedPhaseStillReturnsResult(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{Service: "svc"}

	// Simulate a crash after the deploy phase checkpointed "done" but before the
	// receipt was saved: the journal is active with the phase marked done.
	backend := transactions.NewFileBackend(root)
	resumed := transactions.NewJournal("svc", "dev", "deploy", backend)
	if err := resumed.Checkpoint(CheckpointDeployFunctions, "done"); err != nil {
		t.Fatalf("seed checkpoint: %v", err)
	}

	calls := 0
	res, err := RunDeploy(context.Background(), cfg, "dev", root, FaultConfig{}, resumed, func(context.Context) (*providers.DeployResult, error) {
		calls++
		return &providers.DeployResult{Provider: "test", DeploymentID: "d1"}, nil
	})
	if err != nil {
		t.Fatalf("RunDeploy on resume: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil result on resume, got nil (would panic the caller)")
	}
	if res.DeploymentID != "d1" {
		t.Errorf("result DeploymentID = %q, want d1", res.DeploymentID)
	}
	if calls != 1 {
		t.Errorf("deployFn calls = %d, want 1 (re-run to recover result)", calls)
	}
}

// TestRunDeploy_SecondDeployDoesNotConflictWithCompletedJournal guards against a
// regression where, because the deploy path leaves a completed journal on disk
// at a higher version, a subsequent deploy's fresh journal would be rejected by
// the version conflict check.
func TestRunDeploy_SecondDeployDoesNotConflictWithCompletedJournal(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{Service: "svc"}
	deploy := func(context.Context) (*providers.DeployResult, error) {
		return &providers.DeployResult{Provider: "test"}, nil
	}

	for i := 0; i < 3; i++ {
		journal := OpenDeployJournal("svc", "dev", root)
		if _, err := RunDeploy(context.Background(), cfg, "dev", root, FaultConfig{}, journal, deploy); err != nil {
			t.Fatalf("deploy #%d failed (journal version conflict regression): %v", i+1, err)
		}
	}
}

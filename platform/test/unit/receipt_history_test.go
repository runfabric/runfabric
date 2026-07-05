package unit

import (
	"testing"

	state "github.com/runfabric/runfabric/platform/core/state/core"
)

// Saving a stage twice should retain the outgoing head as one history entry.
func TestReleaseHistory_SnapshotsOnOverwrite(t *testing.T) {
	root := t.TempDir()

	first := &state.Receipt{Service: "svc", Stage: "dev", Provider: "aws-lambda", DeploymentID: "dep-1"}
	if err := state.Save(root, first); err != nil {
		t.Fatal(err)
	}
	// No history yet — nothing has been overwritten.
	if h, err := state.ListStageHistory(root, "dev"); err != nil || len(h) != 0 {
		t.Fatalf("expected empty history after first save, got %d (err=%v)", len(h), err)
	}

	second := &state.Receipt{Service: "svc", Stage: "dev", Provider: "aws-lambda", DeploymentID: "dep-2"}
	if err := state.Save(root, second); err != nil {
		t.Fatal(err)
	}
	hist, err := state.ListStageHistory(root, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if len(hist) != 1 {
		t.Fatalf("expected 1 history entry after overwrite, got %d", len(hist))
	}
	if hist[0].DeploymentID != "dep-1" {
		t.Errorf("expected snapshot of the outgoing head dep-1, got %q", hist[0].DeploymentID)
	}
	// The head is the newest receipt.
	head, err := state.Load(root, "dev")
	if err != nil || head.DeploymentID != "dep-2" {
		t.Fatalf("head should be dep-2, got %+v (err=%v)", head, err)
	}
}

// History is per-stage and returned newest first.
func TestReleaseHistory_NewestFirstPerStage(t *testing.T) {
	root := t.TempDir()
	for _, dep := range []string{"dep-1", "dep-2", "dep-3"} {
		if err := state.Save(root, &state.Receipt{Service: "svc", Stage: "prod", Provider: "aws-lambda", DeploymentID: dep}); err != nil {
			t.Fatal(err)
		}
	}
	// Three saves → two overwrites → two snapshots (dep-1, dep-2).
	hist, err := state.ListStageHistory(root, "prod")
	if err != nil {
		t.Fatal(err)
	}
	if len(hist) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(hist))
	}
	if hist[0].DeploymentID != "dep-2" || hist[1].DeploymentID != "dep-1" {
		t.Errorf("expected newest-first [dep-2, dep-1], got [%s, %s]", hist[0].DeploymentID, hist[1].DeploymentID)
	}
	// A different stage has its own (empty) history.
	if h, err := state.ListStageHistory(root, "dev"); err != nil || len(h) != 0 {
		t.Fatalf("expected empty dev history, got %d (err=%v)", len(h), err)
	}
}

// A retained snapshot can be loaded back in full by id.
func TestReleaseHistory_LoadEntry(t *testing.T) {
	root := t.TempDir()
	if err := state.Save(root, &state.Receipt{
		Service: "svc", Stage: "dev", Provider: "aws-lambda", DeploymentID: "dep-1",
		Outputs: map[string]string{"url": "https://old.example.com"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := state.Save(root, &state.Receipt{Service: "svc", Stage: "dev", Provider: "aws-lambda", DeploymentID: "dep-2"}); err != nil {
		t.Fatal(err)
	}
	hist, err := state.ListStageHistory(root, "dev")
	if err != nil || len(hist) != 1 {
		t.Fatalf("expected 1 history entry, got %d (err=%v)", len(hist), err)
	}
	loaded, err := state.LoadHistoryEntry(root, "dev", hist[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.DeploymentID != "dep-1" || loaded.Outputs["url"] != "https://old.example.com" {
		t.Errorf("loaded snapshot mismatch: %+v", loaded)
	}
}

// Listing history for a stage that was never deployed is empty, not an error.
func TestReleaseHistory_EmptyForUnknownStage(t *testing.T) {
	root := t.TempDir()
	hist, err := state.ListStageHistory(root, "nope")
	if err != nil {
		t.Fatalf("expected no error for unknown stage, got %v", err)
	}
	if len(hist) != 0 {
		t.Errorf("expected empty history, got %d", len(hist))
	}
}

package unit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/runfabric/runfabric/engine/internal/state"
)

func TestSaveWorkflowRun_Nil(t *testing.T) {
	err := state.SaveWorkflowRun(t.TempDir(), nil)
	if err == nil {
		t.Fatal("expected error for nil run")
	}
}

func TestSaveWorkflowRun_NoRunID(t *testing.T) {
	err := state.SaveWorkflowRun(t.TempDir(), &state.WorkflowRun{Stage: "dev", Status: state.RunStatusOK})
	if err == nil {
		t.Fatal("expected error when runId is empty")
	}
}

func TestSaveWorkflowRun_NoStage(t *testing.T) {
	err := state.SaveWorkflowRun(t.TempDir(), &state.WorkflowRun{RunID: "r1", Status: state.RunStatusOK})
	if err == nil {
		t.Fatal("expected error when stage is empty")
	}
}

func TestSaveWorkflowRun_AndList(t *testing.T) {
	root := t.TempDir()
	r := &state.WorkflowRun{
		RunID: "run-1", Service: "svc", Stage: "dev", Status: state.RunStatusOK,
		StartedAt: "2025-01-01T00:00:00Z", EndedAt: "2025-01-01T00:01:00Z",
	}
	if err := state.SaveWorkflowRun(root, r); err != nil {
		t.Fatal(err)
	}
	list, err := state.ListWorkflowRuns(root, "dev", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].RunID != "run-1" {
		t.Errorf("state.ListWorkflowRuns: got %+v", list)
	}
}

func TestListWorkflowRuns_EmptyDir(t *testing.T) {
	root := t.TempDir()
	list, err := state.ListWorkflowRuns(root, "dev", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Errorf("expected no runs, got %d", len(list))
	}
}

func TestListWorkflowRuns_NoDir(t *testing.T) {
	root := t.TempDir()
	list, err := state.ListWorkflowRuns(root, "dev", 10)
	if err != nil {
		t.Fatal(err)
	}
	if list != nil {
		t.Errorf("expected nil when dir missing, got len=%d", len(list))
	}
}

func TestListWorkflowRuns_RespectsLimit(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".runfabric", "runs", "dev")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		r := &state.WorkflowRun{
			RunID: "run-" + string(rune('a'+i)), Service: "s", Stage: "dev", Status: state.RunStatusOK,
			StartedAt: "2025-01-01T12:0" + string(rune('0'+i)) + ":00Z",
		}
		if err := state.SaveWorkflowRun(root, r); err != nil {
			t.Fatal(err)
		}
	}
	list, err := state.ListWorkflowRuns(root, "dev", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Errorf("limit 2: got %d runs", len(list))
	}
}

func TestWorkflowCostFromRuns_Empty(t *testing.T) {
	s := state.WorkflowCostFromRuns(nil)
	if s.RunCount != 0 {
		t.Errorf("RunCount: got %d", s.RunCount)
	}
}

func TestWorkflowCostFromRuns_RunTotals(t *testing.T) {
	runs := []*state.WorkflowRun{
		{TotalInputTokens: 100, TotalOutputTokens: 50, EstimatedCostUSD: 0.01},
		{TotalInputTokens: 200, TotalOutputTokens: 100, EstimatedCostUSD: 0.02},
	}
	s := state.WorkflowCostFromRuns(runs)
	if s.RunCount != 2 || s.TotalInputTokens != 300 || s.TotalOutputTokens != 150 || s.EstimatedCostUSD != 0.03 {
		t.Errorf("WorkflowCostFromRuns: got %+v", s)
	}
}

func TestWorkflowCostFromRuns_NodeTotals(t *testing.T) {
	runs := []*state.WorkflowRun{
		{Nodes: []state.NodeRun{
			{InputTokens: 10, OutputTokens: 5, EstimatedCostUSD: 0.001},
			{InputTokens: 20, OutputTokens: 10, EstimatedCostUSD: 0.002},
		}},
	}
	s := state.WorkflowCostFromRuns(runs)
	if s.RunCount != 1 || s.TotalInputTokens != 30 || s.TotalOutputTokens != 15 || s.EstimatedCostUSD != 0.003 {
		t.Errorf("WorkflowCostFromRuns (nodes): got %+v", s)
	}
}

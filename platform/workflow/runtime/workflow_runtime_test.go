package runtime

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	state "github.com/runfabric/runfabric/platform/core/state/core"
)

type scriptedStepHandler struct {
	mu      sync.Mutex
	results map[string][]error
	calls   map[string]int
	onStep  func(run *state.WorkflowRun, step state.WorkflowStepRun)
}

func (h *scriptedStepHandler) ExecuteStep(ctx context.Context, run *state.WorkflowRun, step state.WorkflowStepRun) (*StepExecutionResult, error) {
	if h.onStep != nil {
		h.onStep(run, step)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.calls == nil {
		h.calls = map[string]int{}
	}
	h.calls[step.StepID]++
	seq := h.results[step.StepID]
	if len(seq) == 0 {
		return &StepExecutionResult{Output: map[string]any{"ok": true}}, nil
	}
	err := seq[0]
	h.results[step.StepID] = seq[1:]
	if err != nil {
		return nil, err
	}
	return &StepExecutionResult{Output: map[string]any{"ok": true}}, nil
}

func TestWorkflowRuntime_RetryThenSuccess(t *testing.T) {
	root := t.TempDir()
	handler := &scriptedStepHandler{
		results: map[string][]error{
			"s1": {errors.New("transient"), nil},
		},
	}
	rt := NewWorkflowRuntime(root, handler)
	rt.Sleep = func(time.Duration) {}

	run, err := rt.StartRun(context.Background(), WorkflowRunSpec{
		Service:      "svc",
		Stage:        "dev",
		Provider:     "aws-lambda",
		WorkflowHash: "wfhash",
		Entrypoint:   "s1",
		Steps: []WorkflowStepSpec{
			{ID: "s1", Kind: "code", MaxAttempts: 3, Backoff: 5 * time.Millisecond},
		},
	})
	if err != nil {
		t.Fatalf("StartRun returned error: %v", err)
	}
	if run.Status != state.RunStatusOK {
		t.Fatalf("expected run status ok, got %q", run.Status)
	}
	if len(run.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(run.Steps))
	}
	if run.Steps[0].AttemptCount != 2 {
		t.Fatalf("expected 2 attempts, got %d", run.Steps[0].AttemptCount)
	}
}

func TestWorkflowRuntime_Timeout(t *testing.T) {
	root := t.TempDir()
	handler := &scriptedStepHandler{
		onStep: func(_ *state.WorkflowRun, _ state.WorkflowStepRun) {
			time.Sleep(30 * time.Millisecond)
		},
	}
	rt := NewWorkflowRuntime(root, handler)
	rt.Sleep = func(time.Duration) {}

	run, err := rt.StartRun(context.Background(), WorkflowRunSpec{
		Service:      "svc",
		Stage:        "dev",
		Provider:     "aws-lambda",
		WorkflowHash: "wfhash",
		Entrypoint:   "s1",
		Steps: []WorkflowStepSpec{
			{ID: "s1", Kind: "code", MaxAttempts: 1, Timeout: 5 * time.Millisecond},
		},
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if run.Status != state.RunStatusTimedOut {
		t.Fatalf("expected run status timed_out, got %q", run.Status)
	}
	if run.Steps[0].Status != state.StepStatusTimedOut {
		t.Fatalf("expected step status timed_out, got %q", run.Steps[0].Status)
	}
}

func TestWorkflowRuntime_Cancel(t *testing.T) {
	root := t.TempDir()
	handler := &scriptedStepHandler{
		onStep: func(run *state.WorkflowRun, step state.WorkflowStepRun) {
			if step.StepID == "s1" {
				_ = state.MarkWorkflowRunCancelRequested(root, run.Stage, run.RunID)
			}
		},
	}
	rt := NewWorkflowRuntime(root, handler)
	rt.Sleep = func(time.Duration) {}

	run, err := rt.StartRun(context.Background(), WorkflowRunSpec{
		Service:      "svc",
		Stage:        "dev",
		Provider:     "aws-lambda",
		WorkflowHash: "wfhash",
		Entrypoint:   "s1",
		Steps: []WorkflowStepSpec{
			{ID: "s1", Kind: "code", MaxAttempts: 1},
			{ID: "s2", Kind: "ai-generate", MaxAttempts: 1},
		},
	})
	if err != nil {
		t.Fatalf("StartRun returned error: %v", err)
	}
	if run.Status != state.RunStatusCancelled {
		t.Fatalf("expected run status cancelled, got %q", run.Status)
	}
	if run.Steps[1].Status != state.StepStatusCancelled {
		t.Fatalf("expected second step status cancelled, got %q", run.Steps[1].Status)
	}
}

func TestWorkflowRuntime_ReplayFromStep(t *testing.T) {
	root := t.TempDir()
	handler := &scriptedStepHandler{results: map[string][]error{}}
	rt := NewWorkflowRuntime(root, handler)
	rt.Sleep = func(time.Duration) {}

	run, err := rt.StartRun(context.Background(), WorkflowRunSpec{
		Service:      "svc",
		Stage:        "dev",
		Provider:     "aws-lambda",
		WorkflowHash: "wfhash",
		Entrypoint:   "s1",
		Steps: []WorkflowStepSpec{
			{ID: "s1", Kind: "code", MaxAttempts: 1},
			{ID: "s2", Kind: "ai-generate", MaxAttempts: 1},
		},
	})
	if err != nil {
		t.Fatalf("StartRun returned error: %v", err)
	}
	firstS1Attempts := run.Steps[0].AttemptCount
	firstS2Attempts := run.Steps[1].AttemptCount

	run, err = rt.ReplayRunFromStep(context.Background(), "dev", run.RunID, "s2")
	if err != nil {
		t.Fatalf("ReplayRunFromStep returned error: %v", err)
	}
	if run.Status != state.RunStatusOK {
		t.Fatalf("expected run status ok after replay, got %q", run.Status)
	}
	if run.Steps[0].AttemptCount != firstS1Attempts {
		t.Fatalf("expected step s1 attempts unchanged (%d), got %d", firstS1Attempts, run.Steps[0].AttemptCount)
	}
	if run.Steps[1].AttemptCount != 1 {
		t.Fatalf("expected replayed step s2 attempt count reset to 1, got %d (first run had %d)", run.Steps[1].AttemptCount, firstS2Attempts)
	}
}

func TestWorkflowRuntime_ResumeAfterRestart(t *testing.T) {
	root := t.TempDir()
	rtCreate := NewWorkflowRuntime(root, &scriptedStepHandler{})
	rtCreate.Sleep = func(time.Duration) {}

	run, err := rtCreate.CreateRun(WorkflowRunSpec{
		RunID:        "resume-run-1",
		Service:      "svc",
		Stage:        "dev",
		Provider:     "aws-lambda",
		WorkflowHash: "wfhash",
		Entrypoint:   "s1",
		Steps: []WorkflowStepSpec{
			{ID: "s1", Kind: "code", MaxAttempts: 1},
			{ID: "s2", Kind: "code", MaxAttempts: 1},
		},
	})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	run.Steps[0].Status = state.StepStatusOK
	run.Steps[0].AttemptCount = 1
	run.Steps[1].Status = state.StepStatusRunning
	run.Steps[1].AttemptCount = 1
	run.Checkpoint = &state.WorkflowCheckpoint{
		CurrentStepID: "s2",
		CurrentStatus: string(state.StepStatusRunning),
		LastError:     "process interrupted",
	}
	if err := state.SaveWorkflowRun(root, run); err != nil {
		t.Fatalf("SaveWorkflowRun returned error: %v", err)
	}

	rtResume := NewWorkflowRuntime(root, &scriptedStepHandler{})
	rtResume.Sleep = func(time.Duration) {}
	resumed, err := rtResume.ResumeRun(context.Background(), "dev", "resume-run-1")
	if err != nil {
		t.Fatalf("ResumeRun returned error: %v", err)
	}
	if resumed.Status != state.RunStatusOK {
		t.Fatalf("expected resumed run status ok, got %q", resumed.Status)
	}
	if resumed.Steps[1].Status != state.StepStatusOK {
		t.Fatalf("expected resumed step s2 status ok, got %q", resumed.Steps[1].Status)
	}
}

// TestWorkflowRuntime_ResumeExhaustedFailedStepKeepsRunFailed guards the
// regression where a crash persisted a step's final failure (AttemptCount ==
// MaxAttempts, status failed) before the run status was finalized: on resume the
// retry loop has no attempts left, and the run was being marked OK instead of
// failed.
func TestWorkflowRuntime_ResumeExhaustedFailedStepKeepsRunFailed(t *testing.T) {
	root := t.TempDir()
	rtCreate := NewWorkflowRuntime(root, &scriptedStepHandler{})
	rtCreate.Sleep = func(time.Duration) {}

	run, err := rtCreate.CreateRun(WorkflowRunSpec{
		RunID:        "resume-failed-1",
		Service:      "svc",
		Stage:        "dev",
		WorkflowHash: "wfhash",
		Entrypoint:   "s1",
		Steps: []WorkflowStepSpec{
			{ID: "s1", Kind: "code", MaxAttempts: 1},
			{ID: "s2", Kind: "code", MaxAttempts: 1},
		},
	})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	// Simulate a crash after the final failed attempt was persisted but before
	// the run status was written: step failed, attempts exhausted, run running.
	run.Steps[0].Status = state.StepStatusFailed
	run.Steps[0].AttemptCount = 1
	run.Steps[0].Error = "boom"
	run.Status = state.RunStatusRunning
	if err := state.SaveWorkflowRun(root, run); err != nil {
		t.Fatalf("SaveWorkflowRun returned error: %v", err)
	}

	// Handler must not be invoked — there are no attempts left.
	handler := &scriptedStepHandler{}
	rtResume := NewWorkflowRuntime(root, handler)
	rtResume.Sleep = func(time.Duration) {}
	resumed, err := rtResume.ResumeRun(context.Background(), "dev", "resume-failed-1")
	if err == nil {
		t.Fatal("expected ResumeRun to return an error for an exhausted failed step")
	}
	if resumed.Status != state.RunStatusFailed {
		t.Fatalf("expected resumed run status failed, got %q", resumed.Status)
	}
	if resumed.Steps[1].Status == state.StepStatusOK {
		t.Fatal("downstream step s2 must not run after an earlier step failed")
	}
	if handler.calls["s1"] != 0 {
		t.Fatalf("handler should not run an exhausted step, got %d calls", handler.calls["s1"])
	}
}

// TestWorkflowRuntime_ReplayWithEarlierFailedStepDoesNotMarkOK guards the
// regression where replaying from a later step skipped earlier steps regardless
// of status, so the run could be marked OK while an earlier step was failed.
func TestWorkflowRuntime_ReplayWithEarlierFailedStepDoesNotMarkOK(t *testing.T) {
	root := t.TempDir()
	rt := NewWorkflowRuntime(root, &scriptedStepHandler{})
	rt.Sleep = func(time.Duration) {}

	run, err := rt.CreateRun(WorkflowRunSpec{
		RunID:   "replay-guard-1",
		Service: "svc",
		Stage:   "dev",
		Steps: []WorkflowStepSpec{
			{ID: "s1", Kind: "code", MaxAttempts: 1},
			{ID: "s2", Kind: "code", MaxAttempts: 1},
		},
	})
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	// Prior state: s1 failed, s2 ok.
	run.Steps[0].Status = state.StepStatusFailed
	run.Steps[0].AttemptCount = 1
	run.Steps[0].Error = "boom"
	run.Steps[1].Status = state.StepStatusOK
	run.Steps[1].AttemptCount = 1
	if err := state.SaveWorkflowRun(root, run); err != nil {
		t.Fatalf("SaveWorkflowRun: %v", err)
	}

	// Replay from s2 only; s1 (failed) is before the replay index and skipped.
	replayed, err := rt.ReplayRunFromStep(context.Background(), "dev", "replay-guard-1", "s2")
	if err == nil {
		t.Fatal("expected error: run cannot succeed with an earlier failed step")
	}
	if replayed.Status == state.RunStatusOK {
		t.Fatalf("run marked OK despite failed earlier step s1; status=%q", replayed.Status)
	}
}

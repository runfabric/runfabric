package controlplane

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	state "github.com/runfabric/runfabric/platform/core/state/core"
)

// WorkflowStepHandler executes one workflow step.
type WorkflowStepHandler interface {
	ExecuteStep(ctx context.Context, run *state.WorkflowRun, step state.WorkflowStepRun) (*StepExecutionResult, error)
}

// StepExecutionResult is the execution envelope returned by step handlers.
type StepExecutionResult struct {
	Output      map[string]any
	Metadata    map[string]any
	Pause       bool
	PauseReason string
}

// WorkflowStepSpec describes one step for a new run.
type WorkflowStepSpec struct {
	ID          string
	Kind        string
	Input       map[string]any
	MaxAttempts int
	Timeout     time.Duration
	Backoff     time.Duration
}

// WorkflowRunSpec describes a workflow run to create and execute.
type WorkflowRunSpec struct {
	RunID        string
	Service      string
	Stage        string
	Provider     string
	WorkflowName string
	WorkflowHash string
	Entrypoint   string
	Steps        []WorkflowStepSpec
}

// WorkflowRuntime provides durable workflow run execution using state.WorkflowRun persistence.
type WorkflowRuntime struct {
	RootDir string
	Handler WorkflowStepHandler
	Now     func() time.Time
	Sleep   func(time.Duration)
}

func NewWorkflowRuntime(rootDir string, handler WorkflowStepHandler) *WorkflowRuntime {
	return &WorkflowRuntime{
		RootDir: rootDir,
		Handler: handler,
		Now:     time.Now,
		Sleep:   time.Sleep,
	}
}

// CreateRun creates a persisted run with pending step records.
func (r *WorkflowRuntime) CreateRun(spec WorkflowRunSpec) (*state.WorkflowRun, error) {
	if r == nil {
		return nil, fmt.Errorf("workflow runtime is nil")
	}
	if spec.Stage == "" {
		return nil, fmt.Errorf("stage is required")
	}
	if spec.Service == "" {
		return nil, fmt.Errorf("service is required")
	}
	runID := spec.RunID
	if runID == "" {
		runID = newWorkflowRunID()
	}
	startedAt := r.nowUTC().Format(time.RFC3339)
	run := &state.WorkflowRun{
		RunID:        runID,
		Service:      spec.Service,
		Stage:        spec.Stage,
		Provider:     spec.Provider,
		WorkflowName: spec.WorkflowName,
		WorkflowHash: spec.WorkflowHash,
		Entrypoint:   spec.Entrypoint,
		Status:       state.RunStatusRunning,
		StartedAt:    startedAt,
		Checkpoint: &state.WorkflowCheckpoint{
			CurrentStatus: string(state.RunStatusRunning),
			UpdatedAt:     startedAt,
		},
	}
	for _, s := range spec.Steps {
		if s.ID == "" {
			return nil, fmt.Errorf("step id is required")
		}
		maxAttempts := s.MaxAttempts
		if maxAttempts <= 0 {
			maxAttempts = 1
		}
		run.Steps = append(run.Steps, state.WorkflowStepRun{
			StepID:      s.ID,
			Kind:        s.Kind,
			Input:       s.Input,
			Status:      state.StepStatusPending,
			MaxAttempts: maxAttempts,
			TimeoutMs:   s.Timeout.Milliseconds(),
			BackoffMs:   s.Backoff.Milliseconds(),
		})
	}
	if len(run.Steps) == 0 {
		now := r.nowUTC()
		run.Status = state.RunStatusOK
		run.EndedAt = now.Format(time.RFC3339)
		run.DurationMs = durationMs(run.StartedAt, run.EndedAt)
	}
	if err := state.SaveWorkflowRun(r.RootDir, run); err != nil {
		return nil, err
	}
	return run, nil
}

// StartRun creates and executes a workflow run.
func (r *WorkflowRuntime) StartRun(ctx context.Context, spec WorkflowRunSpec) (*state.WorkflowRun, error) {
	run, err := r.CreateRun(spec)
	if err != nil {
		return nil, err
	}
	return r.ResumeRun(ctx, run.Stage, run.RunID)
}

// ResumeRun resumes a persisted run from its checkpoint and durable step statuses.
func (r *WorkflowRuntime) ResumeRun(ctx context.Context, stage, runID string) (*state.WorkflowRun, error) {
	if r == nil {
		return nil, fmt.Errorf("workflow runtime is nil")
	}
	if r.Handler == nil {
		return nil, fmt.Errorf("workflow step handler is required")
	}
	run, err := state.LoadWorkflowRun(r.RootDir, stage, runID)
	if err != nil {
		return nil, err
	}
	if state.IsWorkflowRunTerminal(run.Status) && run.ReplayFromStep == "" {
		return run, nil
	}
	if run.Status == "" || run.Status == state.RunStatusPending {
		run.Status = state.RunStatusRunning
		if run.StartedAt == "" {
			run.StartedAt = r.nowUTC().Format(time.RFC3339)
		}
		if err := state.SaveWorkflowRun(r.RootDir, run); err != nil {
			return nil, err
		}
	}

	replayStartIdx := -1
	if run.ReplayFromStep != "" {
		for i := range run.Steps {
			if run.Steps[i].StepID == run.ReplayFromStep {
				replayStartIdx = i
				break
			}
		}
	}

	for idx := range run.Steps {
		run, err = state.LoadWorkflowRun(r.RootDir, stage, runID)
		if err != nil {
			return nil, err
		}

		step := run.Steps[idx]
		if replayStartIdx >= 0 && idx < replayStartIdx {
			continue
		}
		if replayStartIdx < 0 && step.Status == state.StepStatusOK {
			continue
		}
		if step.Status == state.StepStatusPaused {
			decision, _ := step.Input["approvalDecision"].(string)
			if decision == "" {
				run.Status = state.RunStatusPaused
				run.Checkpoint = checkpoint(step.StepID, string(state.StepStatusPaused), "awaiting human approval")
				if err := state.SaveWorkflowRun(r.RootDir, run); err != nil {
					return nil, err
				}
				return run, nil
			}
			run.Steps[idx].Status = state.StepStatusPending
			run.Checkpoint = checkpoint(step.StepID, string(state.StepStatusPending), "approval decision received")
			if err := state.SaveWorkflowRun(r.RootDir, run); err != nil {
				return nil, err
			}
		}
		if step.Status == state.StepStatusRunning {
			run.Steps[idx].Status = state.StepStatusPending
			if run.Steps[idx].AttemptCount > 0 {
				// "running" persisted after restart means the previous in-flight attempt
				// did not reach a terminal transition and should be retried.
				run.Steps[idx].AttemptCount--
			}
			run.Checkpoint = checkpoint(step.StepID, string(state.StepStatusPending), "resumed after restart")
			if err := state.SaveWorkflowRun(r.RootDir, run); err != nil {
				return nil, err
			}
		}
		if run.CancelRequested {
			return r.markCancelled(run, idx)
		}

		run, err = r.executeStep(ctx, run, idx)
		if err != nil {
			return run, err
		}
		if run.Status == state.RunStatusPaused {
			return run, nil
		}
	}

	run.ReplayFromStep = ""
	run.Status = state.RunStatusOK
	run.EndedAt = r.nowUTC().Format(time.RFC3339)
	run.DurationMs = durationMs(run.StartedAt, run.EndedAt)
	run.Checkpoint = checkpoint("", string(state.RunStatusOK), "")
	if err := state.SaveWorkflowRun(r.RootDir, run); err != nil {
		return nil, err
	}
	return run, nil
}

// CancelRun marks a run as cancel-requested. The request is honored on the next transition boundary.
func (r *WorkflowRuntime) CancelRun(stage, runID string) error {
	return state.MarkWorkflowRunCancelRequested(r.RootDir, stage, runID)
}

// ReplayRunFromStep resets durable step state from stepID onward, then re-executes from that step.
func (r *WorkflowRuntime) ReplayRunFromStep(ctx context.Context, stage, runID, stepID string) (*state.WorkflowRun, error) {
	if stepID == "" {
		return nil, fmt.Errorf("step id is required")
	}
	run, err := state.LoadWorkflowRun(r.RootDir, stage, runID)
	if err != nil {
		return nil, err
	}
	startIdx := -1
	for i := range run.Steps {
		if run.Steps[i].StepID == stepID {
			startIdx = i
			break
		}
	}
	if startIdx < 0 {
		return nil, fmt.Errorf("step %q does not exist in run %q", stepID, runID)
	}

	for i := startIdx; i < len(run.Steps); i++ {
		run.Steps[i].Status = state.StepStatusPending
		run.Steps[i].AttemptCount = 0
		run.Steps[i].StartedAt = ""
		run.Steps[i].EndedAt = ""
		run.Steps[i].DurationMs = 0
		run.Steps[i].Error = ""
		run.Steps[i].Output = nil
		run.Steps[i].Metadata = nil
	}
	run.ReplayFromStep = stepID
	run.CancelRequested = false
	run.Status = state.RunStatusRunning
	run.EndedAt = ""
	run.DurationMs = 0
	run.Checkpoint = checkpoint(stepID, string(state.StepStatusPending), "")
	if err := state.SaveWorkflowRun(r.RootDir, run); err != nil {
		return nil, err
	}
	return r.ResumeRun(ctx, stage, runID)
}

func (r *WorkflowRuntime) executeStep(ctx context.Context, run *state.WorkflowRun, idx int) (*state.WorkflowRun, error) {
	step := &run.Steps[idx]
	maxAttempts := step.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	firstAttempt := step.AttemptCount + 1
	for attempt := firstAttempt; attempt <= maxAttempts; attempt++ {
		latest, err := state.LoadWorkflowRun(r.RootDir, run.Stage, run.RunID)
		if err != nil {
			return nil, err
		}
		if latest.CancelRequested {
			return r.markCancelled(latest, idx)
		}
		run = latest
		step = &run.Steps[idx]
		step.MaxAttempts = maxAttempts
		step.AttemptCount = attempt
		step.Status = state.StepStatusRunning
		if step.StartedAt == "" {
			step.StartedAt = r.nowUTC().Format(time.RFC3339)
		}
		step.EndedAt = ""
		step.DurationMs = 0
		step.Error = ""
		run.Status = state.RunStatusRunning
		run.Checkpoint = checkpoint(step.StepID, string(state.StepStatusRunning), "")
		if err := state.SaveWorkflowRun(r.RootDir, run); err != nil {
			return nil, err
		}

		execCtx := ctx
		cancel := func() {}
		if step.TimeoutMs > 0 {
			execCtx, cancel = context.WithTimeout(ctx, time.Duration(step.TimeoutMs)*time.Millisecond)
		}
		res, execErr := r.Handler.ExecuteStep(execCtx, run, *step)
		cancel()

		run, err = state.LoadWorkflowRun(r.RootDir, run.Stage, run.RunID)
		if err != nil {
			return nil, err
		}
		step = &run.Steps[idx]
		step.EndedAt = r.nowUTC().Format(time.RFC3339)
		step.DurationMs = durationMs(step.StartedAt, step.EndedAt)

		if execErr == nil && execCtx.Err() == nil {
			if res != nil && res.Pause {
				step.Status = state.StepStatusPaused
				step.Error = res.PauseReason
				step.Output = res.Output
				step.Metadata = res.Metadata
				run.Status = state.RunStatusPaused
				run.Checkpoint = checkpoint(step.StepID, string(state.StepStatusPaused), res.PauseReason)
				if err := state.SaveWorkflowRun(r.RootDir, run); err != nil {
					return nil, err
				}
				return run, nil
			}
			step.Status = state.StepStatusOK
			step.Error = ""
			if res != nil {
				step.Output = res.Output
				step.Metadata = res.Metadata
			} else {
				step.Output = nil
				step.Metadata = nil
			}
			run.Checkpoint = checkpoint(step.StepID, string(state.StepStatusOK), "")
			if err := state.SaveWorkflowRun(r.RootDir, run); err != nil {
				return nil, err
			}
			return run, nil
		}

		err = execErr
		if err == nil {
			err = execCtx.Err()
		}
		if err == nil {
			err = fmt.Errorf("step execution failed")
		}
		if errors.Is(execCtx.Err(), context.DeadlineExceeded) {
			step.Status = state.StepStatusTimedOut
		} else {
			step.Status = state.StepStatusFailed
		}
		step.Error = err.Error()
		run.Checkpoint = checkpoint(step.StepID, string(step.Status), step.Error)
		if err := state.SaveWorkflowRun(r.RootDir, run); err != nil {
			return nil, err
		}

		if attempt < maxAttempts {
			step.Status = state.StepStatusPending
			run.Checkpoint = checkpoint(step.StepID, string(state.StepStatusPending), step.Error)
			if err := state.SaveWorkflowRun(r.RootDir, run); err != nil {
				return nil, err
			}
			backoff := time.Duration(step.BackoffMs) * time.Millisecond
			if backoff > 0 {
				r.Sleep(backoff)
			}
			continue
		}

		if step.Status == state.StepStatusTimedOut {
			run.Status = state.RunStatusTimedOut
		} else {
			run.Status = state.RunStatusFailed
		}
		run.EndedAt = r.nowUTC().Format(time.RFC3339)
		run.DurationMs = durationMs(run.StartedAt, run.EndedAt)
		if err := state.SaveWorkflowRun(r.RootDir, run); err != nil {
			return nil, err
		}
		return run, fmt.Errorf("workflow step %s failed: %w", step.StepID, err)
	}
	return run, nil
}

func (r *WorkflowRuntime) markCancelled(run *state.WorkflowRun, fromIdx int) (*state.WorkflowRun, error) {
	for i := fromIdx; i < len(run.Steps); i++ {
		if !state.IsWorkflowStepTerminal(run.Steps[i].Status) {
			run.Steps[i].Status = state.StepStatusCancelled
			if run.Steps[i].EndedAt == "" {
				run.Steps[i].EndedAt = r.nowUTC().Format(time.RFC3339)
			}
			if run.Steps[i].StartedAt != "" {
				run.Steps[i].DurationMs = durationMs(run.Steps[i].StartedAt, run.Steps[i].EndedAt)
			}
			if run.Steps[i].Error == "" {
				run.Steps[i].Error = "cancel requested"
			}
		}
	}
	run.Status = state.RunStatusCancelled
	run.EndedAt = r.nowUTC().Format(time.RFC3339)
	run.DurationMs = durationMs(run.StartedAt, run.EndedAt)
	run.Checkpoint = checkpoint("", string(state.RunStatusCancelled), "cancel requested")
	if err := state.SaveWorkflowRun(r.RootDir, run); err != nil {
		return nil, err
	}
	return run, nil
}

// ResolveApproval stores a human approval decision and unpauses the selected step.
func (r *WorkflowRuntime) ResolveApproval(stage, runID, stepID, decision, reviewer string) error {
	run, err := state.LoadWorkflowRun(r.RootDir, stage, runID)
	if err != nil {
		return err
	}
	if decision == "" {
		return fmt.Errorf("approval decision is required")
	}
	for i := range run.Steps {
		if run.Steps[i].StepID != stepID {
			continue
		}
		if run.Steps[i].Input == nil {
			run.Steps[i].Input = map[string]any{}
		}
		run.Steps[i].Input["approvalDecision"] = decision
		if reviewer != "" {
			run.Steps[i].Input["approvalReviewer"] = reviewer
		}
		if run.Steps[i].Status == state.StepStatusPaused {
			run.Steps[i].Status = state.StepStatusPending
		}
		run.Steps[i].AttemptCount = 0
		run.Status = state.RunStatusRunning
		run.Checkpoint = checkpoint(stepID, string(state.StepStatusPending), "")
		return state.SaveWorkflowRun(r.RootDir, run)
	}
	return fmt.Errorf("step %q not found in run %q", stepID, runID)
}

func (r *WorkflowRuntime) nowUTC() time.Time {
	if r.Now == nil {
		return time.Now().UTC()
	}
	return r.Now().UTC()
}

func checkpoint(stepID, status, lastErr string) *state.WorkflowCheckpoint {
	return &state.WorkflowCheckpoint{
		CurrentStepID: stepID,
		CurrentStatus: status,
		UpdatedAt:     time.Now().UTC().Format(time.RFC3339),
		LastError:     lastErr,
	}
}

func durationMs(startedAt, endedAt string) int64 {
	if startedAt == "" || endedAt == "" {
		return 0
	}
	start, err := time.Parse(time.RFC3339, startedAt)
	if err != nil {
		return 0
	}
	end, err := time.Parse(time.RFC3339, endedAt)
	if err != nil {
		return 0
	}
	return end.Sub(start).Milliseconds()
}

func newWorkflowRunID() string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

package runtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	state "github.com/runfabric/runfabric/platform/core/state/core"
)

// runLocks serializes execution of a single run within this process. The
// runtime drives each run as a load-mutate-save loop with no atomic
// compare-and-swap, so two concurrent ResumeRun/Replay calls for the same run
// would interleave and clobber each other's status (steps run twice, attempt
// counters corrupt). This guards against that in-process.
//
// NOTE: this does NOT coordinate across processes/instances. Running the same
// run from multiple instances still requires a shared state+lock backend
// (DynamoDB conditional writes / distributed lock). The local filesystem
// backend is single-instance only.
var runLocks sync.Map // key "stage/runID" -> *sync.Mutex

func lockRun(stage, runID string) func() {
	m, _ := runLocks.LoadOrStore(stage+"/"+runID, &sync.Mutex{})
	mu := m.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

// acquireRunLock takes the run lock, preferring the configured distributed
// Locker (cross-instance) and falling back to the in-process lock. The returned
// release function must always be called.
func (r *WorkflowRuntime) acquireRunLock(ctx context.Context, stage, runID string) (func() error, error) {
	if r.Locker != nil {
		ttl := r.LockTTL
		if ttl <= 0 {
			ttl = 5 * time.Minute
		}
		return r.Locker.Lock(ctx, stage, runID, ttl)
	}
	unlock := lockRun(stage, runID)
	return func() error { unlock(); return nil }, nil
}

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

	// Locker, when set, provides cross-instance mutual exclusion for a run
	// (e.g. a runstore.RunLocker backed by DynamoDB/Redis). When nil, the
	// runtime falls back to an in-process lock, which is correct only for a
	// single instance. See platform/core/state/runstore.
	Locker RunLocker
	// LockTTL bounds how long a crashed holder can wedge a distributed lock
	// before it is treated as stale. Ignored by the in-process fallback.
	LockTTL time.Duration
}

// RunLocker is the subset of runstore.RunLocker the runtime needs. It is
// declared here (rather than imported) so the runtime does not depend on the
// runstore package and to keep the in-process fallback dependency-free.
type RunLocker interface {
	Lock(ctx context.Context, stage, runID string, ttl time.Duration) (release func() error, err error)
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
	release, err := r.acquireRunLock(ctx, stage, runID)
	if err != nil {
		return nil, fmt.Errorf("acquire run lock: %w", err)
	}
	defer func() { _ = release() }()
	return r.resumeRunLocked(ctx, stage, runID)
}

func (r *WorkflowRuntime) resumeRunLocked(ctx context.Context, stage, runID string) (*state.WorkflowRun, error) {
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
				run.Checkpoint = checkpoint(r.nowUTC(), step.StepID, string(state.StepStatusPaused), "awaiting human approval")
				if err := state.SaveWorkflowRun(r.RootDir, run); err != nil {
					return nil, err
				}
				return run, nil
			}
			run.Steps[idx].Status = state.StepStatusPending
			run.Checkpoint = checkpoint(r.nowUTC(), step.StepID, string(state.StepStatusPending), "approval decision received")
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
			run.Checkpoint = checkpoint(r.nowUTC(), step.StepID, string(state.StepStatusPending), "resumed after restart")
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

	// Before declaring success, ensure every step actually succeeded. On a replay
	// from a later step, earlier steps are skipped without re-execution; if one of
	// them is in a non-OK state the run did not succeed and must not be marked OK.
	if failed := firstUnsuccessfulStep(run); failed != nil {
		run.ReplayFromStep = ""
		if failed.Status == state.StepStatusTimedOut {
			run.Status = state.RunStatusTimedOut
		} else {
			run.Status = state.RunStatusFailed
		}
		run.EndedAt = r.nowUTC().Format(time.RFC3339)
		run.DurationMs = durationMs(run.StartedAt, run.EndedAt)
		reason := failed.Error
		if reason == "" {
			reason = fmt.Sprintf("step %s did not complete (status %s)", failed.StepID, failed.Status)
		}
		run.Checkpoint = checkpoint(r.nowUTC(), failed.StepID, string(failed.Status), reason)
		if err := state.SaveWorkflowRun(r.RootDir, run); err != nil {
			return nil, err
		}
		return run, fmt.Errorf("workflow step %s did not succeed (status %s)", failed.StepID, failed.Status)
	}

	run.ReplayFromStep = ""
	run.Status = state.RunStatusOK
	run.EndedAt = r.nowUTC().Format(time.RFC3339)
	run.DurationMs = durationMs(run.StartedAt, run.EndedAt)
	run.Checkpoint = checkpoint(r.nowUTC(), "", string(state.RunStatusOK), "")
	if err := state.SaveWorkflowRun(r.RootDir, run); err != nil {
		return nil, err
	}
	return run, nil
}

// firstUnsuccessfulStep returns a pointer to the first step that is not OK, or
// nil when every step succeeded.
func firstUnsuccessfulStep(run *state.WorkflowRun) *state.WorkflowStepRun {
	for i := range run.Steps {
		if run.Steps[i].Status != state.StepStatusOK {
			return &run.Steps[i]
		}
	}
	return nil
}

// CancelRun marks a run as cancel-requested. The request is honored on the next
// transition boundary. It deliberately does NOT take the per-run lock: a cancel
// must be writable while ResumeRun holds the lock for the whole execution, and
// executeStep reloads the run each attempt to observe the flag.
func (r *WorkflowRuntime) CancelRun(stage, runID string) error {
	return state.MarkWorkflowRunCancelRequested(r.RootDir, stage, runID)
}

// ReplayRunFromStep resets durable step state from stepID onward, then re-executes from that step.
func (r *WorkflowRuntime) ReplayRunFromStep(ctx context.Context, stage, runID, stepID string) (*state.WorkflowRun, error) {
	if stepID == "" {
		return nil, fmt.Errorf("step id is required")
	}
	release, err := r.acquireRunLock(ctx, stage, runID)
	if err != nil {
		return nil, fmt.Errorf("acquire run lock: %w", err)
	}
	defer func() { _ = release() }()
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
	run.Checkpoint = checkpoint(r.nowUTC(), stepID, string(state.StepStatusPending), "")
	if err := state.SaveWorkflowRun(r.RootDir, run); err != nil {
		return nil, err
	}
	// Already holding the run lock; call the locked variant to avoid re-locking.
	return r.resumeRunLocked(ctx, stage, runID)
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
		run.Checkpoint = checkpoint(r.nowUTC(), step.StepID, string(state.StepStatusRunning), "")
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
				run.Checkpoint = checkpoint(r.nowUTC(), step.StepID, string(state.StepStatusPaused), res.PauseReason)
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
			run.Checkpoint = checkpoint(r.nowUTC(), step.StepID, string(state.StepStatusOK), "")
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
		run.Checkpoint = checkpoint(r.nowUTC(), step.StepID, string(step.Status), step.Error)
		if err := state.SaveWorkflowRun(r.RootDir, run); err != nil {
			return nil, err
		}

		if attempt < maxAttempts {
			step.Status = state.StepStatusPending
			run.Checkpoint = checkpoint(r.nowUTC(), step.StepID, string(state.StepStatusPending), step.Error)
			if err := state.SaveWorkflowRun(r.RootDir, run); err != nil {
				return nil, err
			}
			backoff := time.Duration(step.BackoffMs) * time.Millisecond
			if backoff > 0 {
				r.Sleep(backoff)
			}
			// Stop retrying if the run context was cancelled or timed out during
			// the attempt/backoff instead of pressing on with a doomed retry.
			if ctxErr := ctx.Err(); ctxErr != nil {
				if errors.Is(ctxErr, context.DeadlineExceeded) {
					step.Status = state.StepStatusTimedOut
					run.Status = state.RunStatusTimedOut
				} else {
					step.Status = state.StepStatusCancelled
					run.Status = state.RunStatusCancelled
				}
				step.Error = ctxErr.Error()
				run.EndedAt = r.nowUTC().Format(time.RFC3339)
				run.DurationMs = durationMs(run.StartedAt, run.EndedAt)
				run.Checkpoint = checkpoint(r.nowUTC(), step.StepID, string(step.Status), ctxErr.Error())
				if err := state.SaveWorkflowRun(r.RootDir, run); err != nil {
					return nil, err
				}
				return run, fmt.Errorf("workflow step %s aborted: %w", step.StepID, ctxErr)
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

	// The retry loop did not execute because this step's attempts were already
	// exhausted before resume (firstAttempt > maxAttempts). If the persisted step
	// is in a terminal failure state, the run failed at this step — a crash can
	// persist the final step failure before the run status is finalized.
	// Reconcile the run status instead of falsely reporting success.
	final := &run.Steps[idx]
	if final.Status == state.StepStatusFailed || final.Status == state.StepStatusTimedOut {
		if final.Status == state.StepStatusTimedOut {
			run.Status = state.RunStatusTimedOut
		} else {
			run.Status = state.RunStatusFailed
		}
		run.EndedAt = r.nowUTC().Format(time.RFC3339)
		run.DurationMs = durationMs(run.StartedAt, run.EndedAt)
		run.Checkpoint = checkpoint(r.nowUTC(), final.StepID, string(final.Status), final.Error)
		if err := state.SaveWorkflowRun(r.RootDir, run); err != nil {
			return nil, err
		}
		return run, fmt.Errorf("workflow step %s failed: %s", final.StepID, final.Error)
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
	run.Checkpoint = checkpoint(r.nowUTC(), "", string(state.RunStatusCancelled), "cancel requested")
	if err := state.SaveWorkflowRun(r.RootDir, run); err != nil {
		return nil, err
	}
	return run, nil
}

// ResolveApproval stores a human approval decision and unpauses the selected step.
func (r *WorkflowRuntime) ResolveApproval(stage, runID, stepID, decision, reviewer string) error {
	release, err := r.acquireRunLock(context.Background(), stage, runID)
	if err != nil {
		return fmt.Errorf("acquire run lock: %w", err)
	}
	defer func() { _ = release() }()
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
		run.Checkpoint = checkpoint(r.nowUTC(), stepID, string(state.StepStatusPending), "")
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

func checkpoint(now time.Time, stepID, status, lastErr string) *state.WorkflowCheckpoint {
	return &state.WorkflowCheckpoint{
		CurrentStepID: stepID,
		CurrentStatus: status,
		UpdatedAt:     now.UTC().Format(time.RFC3339),
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

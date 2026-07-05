package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/runfabric/runfabric/platform/core/model/config"
	state "github.com/runfabric/runfabric/platform/core/state/core"
	"github.com/runfabric/runfabric/platform/core/state/runstore"
	workflowruntime "github.com/runfabric/runfabric/platform/workflow/runtime"
)

// newConfiguredRuntime builds a WorkflowRuntime whose run lock comes from the
// user-selected backend. The store is chosen by RUNFABRIC_RUN_STORE or
// extensions.runStore in runfabric.yml (default: local). Choosing the store
// also chooses how runs are locked — the backend supplies its own RunLocker.
//
// NOTE: run-state persistence still goes through the local state files; a remote
// store currently provides only the lock. Migrating SaveWorkflowRun/LoadWorkflowRun
// onto RunStore (with CAS) is the remaining step for full multi-instance runs.
func newConfiguredRuntime(cfg *config.Config, root string, handler workflowruntime.WorkflowStepHandler) (*workflowruntime.WorkflowRuntime, error) {
	store, err := runstore.Resolve(config.ExtensionString(cfg, "runStore"), root)
	if err != nil {
		return nil, fmt.Errorf("resolve run store: %w", err)
	}
	rt := workflowruntime.NewWorkflowRuntime(root, handler)
	rt.Locker = runstore.LockerFor(store)
	rt.Store = store
	return rt, nil
}

type WorkflowRunResult struct {
	Workflow string             `json:"workflow"`
	Source   string             `json:"source"`
	Warnings []string           `json:"warnings,omitempty"`
	Run      *state.WorkflowRun `json:"run"`
}

func WorkflowRun(configPath, stage, providerOverride, workflowName, runID string, runInput map[string]any) (*WorkflowRunResult, error) {
	ctx, err := Bootstrap(configPath, stage, providerOverride)
	if err != nil {
		return nil, err
	}

	spec, source, warnings, err := buildWorkflowRunSpec(ctx.Config, workflowName, runID, runInput)
	if err != nil {
		return nil, err
	}
	spec.Service = ctx.Config.Service
	spec.Stage = stage
	spec.Provider = ctx.Config.Provider.Name

	handler, err := workflowruntime.NewTypedStepHandlerFromConfig(ctx.Config, nil)
	if err != nil {
		return nil, err
	}
	handler.CodeRunner = newInvokeCodeStepRunner(ctx)
	runtime, err := newConfiguredRuntime(ctx.Config, ctx.RootDir, handler)
	if err != nil {
		return nil, err
	}
	run, runErr := runtime.StartRun(context.Background(), spec)
	res := &WorkflowRunResult{
		Workflow: spec.WorkflowName,
		Source:   source,
		Warnings: warnings,
		Run:      run,
	}
	if runErr != nil {
		return res, runErr
	}
	return res, nil
}

func WorkflowStatus(configPath, stage, runID string) (*state.WorkflowRun, error) {
	// Bootstrap so the configured run store (extensions.runStore / env) is used;
	// a dynamodb:// deployment must read status from the same backend the runtime
	// wrote to, not from empty local files.
	ctx, err := Bootstrap(configPath, stage, "")
	if err != nil {
		return nil, err
	}
	return loadRunVia(ctx.Config, ctx.RootDir, stage, runID)
}

func WorkflowCancel(configPath, stage, runID string) (*state.WorkflowRun, error) {
	ctx, err := Bootstrap(configPath, stage, "")
	if err != nil {
		return nil, err
	}
	rt, err := newConfiguredRuntime(ctx.Config, ctx.RootDir, nil)
	if err != nil {
		return nil, err
	}
	if err := rt.CancelRun(stage, runID); err != nil {
		return nil, err
	}
	return loadRunVia(ctx.Config, ctx.RootDir, stage, runID)
}

func WorkflowReplay(configPath, stage, providerOverride, runID, stepID string) (*state.WorkflowRun, error) {
	ctx, err := Bootstrap(configPath, stage, providerOverride)
	if err != nil {
		return nil, err
	}
	handler, err := workflowruntime.NewTypedStepHandlerFromConfig(ctx.Config, nil)
	if err != nil {
		return nil, err
	}
	handler.CodeRunner = newInvokeCodeStepRunner(ctx)
	runtime, err := newConfiguredRuntime(ctx.Config, ctx.RootDir, handler)
	if err != nil {
		return nil, err
	}
	return runtime.ReplayRunFromStep(context.Background(), stage, runID, stepID)
}

// WorkflowApprove records a human-approval decision (approve|reject) for a
// paused step and resumes the run so it advances past the gate. stepID is
// optional: when empty it targets the run's currently paused step. Returns the
// resulting run (still paused if a later step also awaits approval).
func WorkflowApprove(configPath, stage, providerOverride, runID, stepID, decision, reviewer string) (*state.WorkflowRun, error) {
	ctx, err := Bootstrap(configPath, stage, providerOverride)
	if err != nil {
		return nil, err
	}
	handler, err := workflowruntime.NewTypedStepHandlerFromConfig(ctx.Config, nil)
	if err != nil {
		return nil, err
	}
	handler.CodeRunner = newInvokeCodeStepRunner(ctx)
	runtime, err := newConfiguredRuntime(ctx.Config, ctx.RootDir, handler)
	if err != nil {
		return nil, err
	}
	// Default to the paused step so callers need only the run id + decision.
	if strings.TrimSpace(stepID) == "" {
		run, err := loadRunVia(ctx.Config, ctx.RootDir, stage, runID)
		if err != nil {
			return nil, err
		}
		stepID = pausedStepID(run)
		if stepID == "" {
			return nil, fmt.Errorf("run %q has no step awaiting approval", runID)
		}
	}
	if err := runtime.ResolveApproval(stage, runID, stepID, decision, reviewer); err != nil {
		return nil, err
	}
	return runtime.ResumeRun(context.Background(), stage, runID)
}

// pausedStepID returns the id of the run's paused step (checkpoint first, then a
// step scan), or "" when none is awaiting approval.
func pausedStepID(run *state.WorkflowRun) string {
	if run == nil {
		return ""
	}
	if run.Checkpoint != nil && run.Checkpoint.CurrentStatus == string(state.StepStatusPaused) {
		return run.Checkpoint.CurrentStepID
	}
	for _, step := range run.Steps {
		if step.Status == state.StepStatusPaused {
			return step.StepID
		}
	}
	return ""
}

func buildWorkflowRunSpec(cfg *config.Config, workflowName, runID string, runInput map[string]any) (workflowruntime.WorkflowRunSpec, string, []string, error) {
	name := strings.TrimSpace(workflowName)
	if name == "" {
		name = defaultWorkflowName(cfg)
	}
	if name == "" {
		return workflowruntime.WorkflowRunSpec{}, "", nil, fmt.Errorf("workflow name is required (set --name) because multiple/no workflows are configured")
	}

	if len(cfg.Workflows) > 0 {
		if steps, entrypoint, err := buildStepsFromConfiguredWorkflows(cfg.Workflows, name, runInput); err == nil {
			return workflowruntime.WorkflowRunSpec{
				RunID:        runID,
				WorkflowName: name,
				WorkflowHash: workflowHashFromSteps(name, steps),
				Entrypoint:   entrypoint,
				Steps:        steps,
			}, "workflows", nil, nil
		}
	}

	available := availableWorkflowNames(cfg)
	if len(available) == 0 {
		return workflowruntime.WorkflowRunSpec{}, "", nil, fmt.Errorf("no workflows configured")
	}
	return workflowruntime.WorkflowRunSpec{}, "", nil, fmt.Errorf("workflow %q not found (available: %s)", name, strings.Join(available, ", "))
}

func defaultWorkflowName(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	if len(cfg.Workflows) == 1 {
		return strings.TrimSpace(cfg.Workflows[0].Name)
	}
	return ""
}

func availableWorkflowNames(cfg *config.Config) []string {
	names := []string{}
	for _, wf := range cfg.Workflows {
		name := strings.TrimSpace(wf.Name)
		if name != "" {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return names
	}
	seen := map[string]bool{}
	uniq := make([]string, 0, len(names))
	for _, n := range names {
		if seen[n] {
			continue
		}
		seen[n] = true
		uniq = append(uniq, n)
	}
	sort.Strings(uniq)
	return uniq
}

func buildStepsFromConfiguredWorkflows(workflows []config.WorkflowConfig, workflowName string, runInput map[string]any) ([]workflowruntime.WorkflowStepSpec, string, error) {
	for _, wf := range workflows {
		if strings.TrimSpace(wf.Name) != workflowName {
			continue
		}
		steps := make([]workflowruntime.WorkflowStepSpec, 0, len(wf.Steps))
		for i, step := range wf.Steps {
			stepID := strings.TrimSpace(step.ID)
			if stepID == "" {
				return nil, "", fmt.Errorf("workflow %q step %d: id is required", workflowName, i+1)
			}

			kind := strings.TrimSpace(step.Kind)
			if kind == "" {
				return nil, "", fmt.Errorf("workflow %q step %q: kind is required", workflowName, stepID)
			}

			input := map[string]any{}
			for k, v := range step.Input {
				input[k] = v
			}
			copyInputIfAbsent(input, "model", strings.TrimSpace(step.Model))

			maxAttempts := 1
			backoff := time.Duration(0)

			timeout := time.Duration(0)
			if step.Timeout > 0 {
				timeout = time.Duration(step.Timeout) * time.Second
			}

			if i == 0 {
				input = mergeRunInput(input, runInput)
			}
			steps = append(steps, workflowruntime.WorkflowStepSpec{
				ID:          stepID,
				Kind:        kind,
				Input:       input,
				MaxAttempts: maxAttempts,
				Timeout:     timeout,
				Backoff:     backoff,
			})
		}
		entrypoint := ""
		if len(steps) > 0 {
			entrypoint = steps[0].ID
		}
		return steps, entrypoint, nil
	}

	return nil, "", fmt.Errorf("workflow %q not found in configured workflows", workflowName)
}

func mergeRunInput(stepInput, runInput map[string]any) map[string]any {
	if len(runInput) == 0 {
		return stepInput
	}
	if stepInput == nil {
		stepInput = map[string]any{}
	}
	stepInput["runInput"] = runInput
	for k, v := range runInput {
		if _, exists := stepInput[k]; !exists {
			stepInput[k] = v
		}
	}
	return stepInput
}

func copyInputIfAbsent(dst map[string]any, key string, value any) {
	if dst == nil || key == "" || value == nil {
		return
	}
	if _, exists := dst[key]; exists {
		return
	}
	dst[key] = value
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	switch tv := v.(type) {
	case string:
		return strings.TrimSpace(tv)
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
}

func workflowHashFromSteps(name string, steps []workflowruntime.WorkflowStepSpec) string {
	payload := map[string]any{
		"name":  name,
		"steps": steps,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return name
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:12])
}

func asInt(v any) int {
	switch tv := v.(type) {
	case int:
		return tv
	case int64:
		return int(tv)
	case int32:
		return int(tv)
	case float64:
		return int(tv)
	case float32:
		return int(tv)
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(tv))
		return n
	default:
		return 0
	}
}

func asInt64(v any) int64 {
	switch tv := v.(type) {
	case int:
		return int64(tv)
	case int64:
		return tv
	case int32:
		return int64(tv)
	case float64:
		return int64(tv)
	case float32:
		return int64(tv)
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(tv), 10, 64)
		return n
	default:
		return 0
	}
}

func parseDurationLike(v any) (time.Duration, bool) {
	if v == nil {
		return 0, false
	}
	switch tv := v.(type) {
	case string:
		s := strings.TrimSpace(tv)
		if s == "" {
			return 0, false
		}
		d, err := time.ParseDuration(s)
		if err != nil {
			return 0, false
		}
		return d, true
	case int:
		return time.Duration(tv) * time.Second, true
	case int64:
		return time.Duration(tv) * time.Second, true
	case float64:
		return time.Duration(tv * float64(time.Second)), true
	default:
		return 0, false
	}
}

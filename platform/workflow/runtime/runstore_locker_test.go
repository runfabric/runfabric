package runtime_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	corestate "github.com/runfabric/runfabric/platform/core/state/core"
	"github.com/runfabric/runfabric/platform/core/state/runstore"
	runtime "github.com/runfabric/runfabric/platform/workflow/runtime"
)

// A runstore.RunLocker must satisfy the runtime's RunLocker seam, so a
// distributed backend can be dropped in to coordinate runs across instances.
var _ runtime.RunLocker = (runstore.RunLocker)(nil)

type noopHandler struct{}

func (noopHandler) ExecuteStep(context.Context, *corestate.WorkflowRun, corestate.WorkflowStepRun) (*runtime.StepExecutionResult, error) {
	return &runtime.StepExecutionResult{}, nil
}

func TestRuntimeUsesRunStoreLocker(t *testing.T) {
	dir := t.TempDir()
	store := runstore.NewLocalRunStore(dir)

	rt := runtime.NewWorkflowRuntime(dir, noopHandler{})
	rt.Locker = store // inject the shared store as the cross-instance run locker

	run, err := rt.StartRun(context.Background(), runtime.WorkflowRunSpec{
		Service:      "svc",
		Stage:        "dev",
		WorkflowName: "wf",
		WorkflowHash: "h1",
		Steps:        []runtime.WorkflowStepSpec{{ID: "s1", Kind: "noop"}},
	})
	if err != nil {
		t.Fatalf("StartRun with runstore locker: %v", err)
	}
	if run == nil || run.RunID == "" {
		t.Fatal("expected a persisted run")
	}
	if run.Status != corestate.RunStatusOK {
		t.Fatalf("run status = %q, want ok", run.Status)
	}
}

// countingHandler records how many times a step is executed and holds the step
// briefly to widen the overlap window between concurrent resumers.
type countingHandler struct{ calls *int32 }

func (h countingHandler) ExecuteStep(context.Context, *corestate.WorkflowRun, corestate.WorkflowStepRun) (*runtime.StepExecutionResult, error) {
	atomic.AddInt32(h.calls, 1)
	time.Sleep(15 * time.Millisecond)
	return &runtime.StepExecutionResult{}, nil
}

// TestSharedStoreLockPreventsDoubleExecution is the process-level takeover
// guarantee: two runtime instances sharing one run Store + Locker must not both
// drive the same run. The run lock serializes them; whichever loses the race
// observes the terminal run and does not re-execute steps. Each step runs
// exactly once.
//
// NOTE: the kill-a-holder-mid-run / lease-expiry-takeover variant requires a
// distributed backend with a real TTL lease (LocalRunStore's lock is an
// in-process mutex that cannot expire across a crash). That test is gated on the
// DynamoDB run-store backend (currently a fail-closed scaffold) and its
// DynamoDB-Local harness (RUNFABRIC_TEST_DYNAMODB_ENDPOINT).
func TestSharedStoreLockPreventsDoubleExecution(t *testing.T) {
	dir := t.TempDir()
	store := runstore.NewLocalRunStore(dir)
	var calls int32

	newRT := func() *runtime.WorkflowRuntime {
		rt := runtime.NewWorkflowRuntime(dir, countingHandler{calls: &calls})
		rt.Store = store  // shared persistence
		rt.Locker = store // shared lock
		return rt
	}

	run, err := newRT().CreateRun(runtime.WorkflowRunSpec{
		Service:      "svc",
		Stage:        "dev",
		WorkflowHash: "wf",
		Steps: []runtime.WorkflowStepSpec{
			{ID: "s1", Kind: "noop"},
			{ID: "s2", Kind: "noop"},
		},
	})
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = newRT().ResumeRun(context.Background(), run.Stage, run.RunID)
		}()
	}
	wg.Wait()

	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("steps executed %d times, want 2 (each of 2 steps once; the shared lock must prevent double execution)", got)
	}

	final, _, err := store.Load(context.Background(), run.Stage, run.RunID)
	if err != nil {
		t.Fatalf("load final: %v", err)
	}
	if final.Status != corestate.RunStatusOK {
		t.Fatalf("final status = %q, want ok", final.Status)
	}
}

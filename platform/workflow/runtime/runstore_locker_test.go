package runtime_test

import (
	"context"
	"testing"

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

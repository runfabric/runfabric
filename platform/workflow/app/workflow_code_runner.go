package app

import (
	"encoding/json"
	"fmt"
	"strings"

	state "github.com/runfabric/runfabric/platform/core/state/core"
	workflowruntime "github.com/runfabric/runfabric/platform/workflow/runtime"
)

// invokeCodeStepRunner executes `kind: code` workflow steps by invoking the
// deployed function named in input.function through the app invoke dispatch —
// the same path as `runfabric invoke`. input.payload (any JSON-marshalable
// value) is sent as the invocation payload; it defaults to {}.
//
// Steps without input.function keep the previous echo behavior via
// DefaultCodeStepRunner, so existing workflows are unaffected.
type invokeCodeStepRunner struct {
	invoke func(function string, payload []byte) (any, error)
}

var _ workflowruntime.CodeStepRunner = (*invokeCodeStepRunner)(nil)

func newInvokeCodeStepRunner(ctx *AppContext) *invokeCodeStepRunner {
	return &invokeCodeStepRunner{
		invoke: func(function string, payload []byte) (any, error) {
			return invokeWithContext(ctx, function, payload)
		},
	}
}

func (r *invokeCodeStepRunner) ExecuteStep(run *state.WorkflowRun, step state.WorkflowStepRun, output, metadata map[string]any) (*workflowruntime.StepExecutionResult, error) {
	function, _ := step.Input["function"].(string)
	function = strings.TrimSpace(function)
	if function == "" {
		return workflowruntime.DefaultCodeStepRunner{}.ExecuteStep(run, step, output, metadata)
	}

	payload := []byte("{}")
	if raw, ok := step.Input["payload"]; ok && raw != nil {
		data, err := json.Marshal(raw)
		if err != nil {
			return nil, fmt.Errorf("step %s: marshal input.payload: %w", step.StepID, err)
		}
		payload = data
	}

	res, err := r.invoke(function, payload)
	if err != nil {
		return nil, fmt.Errorf("step %s: invoke function %q: %w", step.StepID, function, err)
	}
	output["function"] = function
	output["result"] = res
	return &workflowruntime.StepExecutionResult{Output: output, Metadata: metadata}, nil
}

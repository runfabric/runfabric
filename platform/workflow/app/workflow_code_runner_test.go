package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	state "github.com/runfabric/runfabric/platform/core/state/core"
)

func codeStep(input map[string]any) state.WorkflowStepRun {
	return state.WorkflowStepRun{StepID: "s1", Kind: "code", Input: input}
}

func TestCodeRunnerFallsBackWithoutFunction(t *testing.T) {
	r := &invokeCodeStepRunner{invoke: func(context.Context, string, []byte) (any, error) {
		t.Fatal("invoke must not be called without input.function")
		return nil, nil
	}}
	out := map[string]any{}
	res, err := r.ExecuteStep(context.Background(), nil, codeStep(nil), out, map[string]any{})
	if err != nil {
		t.Fatalf("ExecuteStep: %v", err)
	}
	if res.Output["result"] != "code_executed" {
		t.Fatalf("fallback output = %v, want echo behavior", res.Output)
	}
}

func TestCodeRunnerInvokesFunctionWithPayload(t *testing.T) {
	var gotFunction, gotPayload string
	r := &invokeCodeStepRunner{invoke: func(_ context.Context, function string, payload []byte) (any, error) {
		gotFunction, gotPayload = function, string(payload)
		return map[string]any{"ok": true}, nil
	}}

	out := map[string]any{}
	res, err := r.ExecuteStep(context.Background(), nil, codeStep(map[string]any{
		"function": "api",
		"payload":  map[string]any{"orderId": 7},
	}), out, map[string]any{})
	if err != nil {
		t.Fatalf("ExecuteStep: %v", err)
	}
	if gotFunction != "api" || gotPayload != `{"orderId":7}` {
		t.Fatalf("invoked %q with %q", gotFunction, gotPayload)
	}
	if res.Output["function"] != "api" || res.Output["result"] == nil {
		t.Fatalf("output = %v", res.Output)
	}
}

func TestCodeRunnerDefaultsPayloadToEmptyObject(t *testing.T) {
	var gotPayload string
	r := &invokeCodeStepRunner{invoke: func(_ context.Context, _ string, payload []byte) (any, error) {
		gotPayload = string(payload)
		return "ok", nil
	}}
	if _, err := r.ExecuteStep(context.Background(), nil, codeStep(map[string]any{"function": "api"}), map[string]any{}, map[string]any{}); err != nil {
		t.Fatalf("ExecuteStep: %v", err)
	}
	if gotPayload != "{}" {
		t.Fatalf("payload = %q, want {}", gotPayload)
	}
}

func TestCodeRunnerPropagatesInvokeError(t *testing.T) {
	boom := errors.New("provider unavailable")
	r := &invokeCodeStepRunner{invoke: func(context.Context, string, []byte) (any, error) {
		return nil, boom
	}}
	_, err := r.ExecuteStep(context.Background(), nil, codeStep(map[string]any{"function": "api"}), map[string]any{}, map[string]any{})
	if err == nil || !errors.Is(err, boom) {
		t.Fatalf("ExecuteStep error = %v, want wrapped invoke error", err)
	}
	if !strings.Contains(err.Error(), `function "api"`) {
		t.Fatalf("error %q should name the function", err)
	}
}

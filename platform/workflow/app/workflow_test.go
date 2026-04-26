package app

import (
	"testing"

	"github.com/runfabric/runfabric/platform/core/model/config"
	workflowruntime "github.com/runfabric/runfabric/platform/workflow/runtime"
)

func TestBuildStepsFromConfiguredWorkflows_MapsStepModelOverride(t *testing.T) {
	workflows := []config.WorkflowConfig{
		{
			Name: "release-flow",
			Steps: []config.WorkflowStep{
				{
					ID:    "generate",
					Kind:  workflowruntime.StepKindAIGenerate,
					Model: "top-level-model",
					Input: map[string]any{
						"prompt": "create release notes",
					},
				},
				{
					ID:    "eval",
					Kind:  workflowruntime.StepKindAIEval,
					Model: "top-level-eval-model",
					Input: map[string]any{
						"score": 0.8,
						"model": "input-model",
					},
				},
			},
		},
	}

	steps, _, err := buildStepsFromConfiguredWorkflows(workflows, "release-flow", nil)
	if err != nil {
		t.Fatalf("buildStepsFromConfiguredWorkflows returned error: %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(steps))
	}
	if got, _ := steps[0].Input["model"].(string); got != "top-level-model" {
		t.Fatalf("expected top-level model to map into step input, got %q", got)
	}
	if got, _ := steps[1].Input["model"].(string); got != "input-model" {
		t.Fatalf("expected input.model to take precedence over top-level model, got %q", got)
	}
}

func TestBuildStepsFromConfiguredWorkflows_UsesExplicitID(t *testing.T) {
	workflows := []config.WorkflowConfig{
		{
			Name: "release-flow",
			Steps: []config.WorkflowStep{
				{
					ID:   "plan",
					Kind: workflowruntime.StepKindAIGenerate,
					Input: map[string]any{
						"prompt": "draft plan",
					},
				},
			},
		},
	}

	steps, _, err := buildStepsFromConfiguredWorkflows(workflows, "release-flow", nil)
	if err != nil {
		t.Fatalf("buildStepsFromConfiguredWorkflows returned error: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	if steps[0].ID != "plan" {
		t.Fatalf("expected explicit step id to win, got %q", steps[0].ID)
	}
}

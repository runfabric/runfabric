package app

import (
	"testing"

	"github.com/runfabric/runfabric/platform/core/model/config"
	"github.com/runfabric/runfabric/platform/deploy/controlplane"
)

func TestBuildStepsFromConfiguredWorkflows_MapsStepModelOverride(t *testing.T) {
	workflows := []config.WorkflowConfig{
		{
			Name: "release-flow",
			Steps: []config.WorkflowStep{
				{
					Function: "generate",
					Kind:     controlplane.StepKindAIGenerate,
					Model:    "top-level-model",
					Input: map[string]any{
						"prompt": "create release notes",
					},
				},
				{
					Function: "eval",
					Kind:     controlplane.StepKindAIEval,
					Model:    "top-level-eval-model",
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

func TestBuildStepsFromConfiguredWorkflows_UsesIDBeforeLegacyFunction(t *testing.T) {
	workflows := []config.WorkflowConfig{
		{
			Name: "release-flow",
			Steps: []config.WorkflowStep{
				{
					ID:       "plan",
					Function: "legacy-function-name",
					Kind:     controlplane.StepKindAIGenerate,
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
	if got, _ := steps[0].Input["function"].(string); got != "legacy-function-name" {
		t.Fatalf("expected legacy function to remain available in input, got %q", got)
	}
}

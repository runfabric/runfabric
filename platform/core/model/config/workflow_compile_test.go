package config

import "testing"

func TestCompileWorkflowGraphFromConfig_UsesOrderedStepEdges(t *testing.T) {
	cfg := &Config{
		Workflows: []WorkflowConfig{
			{
				Name: "release-flow",
				Steps: []WorkflowStep{
					{ID: "plan", Kind: "code"},
					{ID: "approve", Kind: "human-approval"},
				},
			},
		},
	}

	graph, err := CompileWorkflowGraphFromConfig(cfg)
	if err != nil {
		t.Fatalf("CompileWorkflowGraphFromConfig returned error: %v", err)
	}
	if graph == nil {
		t.Fatal("expected compiled graph")
	}
	if graph.Entrypoint != "release-flow:plan" {
		t.Fatalf("expected entrypoint to use explicit id, got %q", graph.Entrypoint)
	}
	found := false
	for _, e := range graph.Edges {
		if e.From == "release-flow:plan" && e.To == "release-flow:approve" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected edge plan->approve in compiled graph, got %+v", graph.Edges)
	}
}

func TestCompileWorkflowGraphFromConfig_RequiresStepKind(t *testing.T) {
	cfg := &Config{
		Workflows: []WorkflowConfig{
			{
				Name: "release-flow",
				Steps: []WorkflowStep{
					{ID: "plan"},
				},
			},
		},
	}

	_, err := CompileWorkflowGraphFromConfig(cfg)
	if err == nil {
		t.Fatal("expected missing step kind to fail compile")
	}
}

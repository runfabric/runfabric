package config

import (
	"github.com/runfabric/runfabric/engine/internal/aiflow"
)

// CompileAiWorkflow converts AiWorkflowConfig to aiflow.GraphInput and compiles the DAG. Returns nil, nil when aw is nil or enable is false.
func CompileAiWorkflow(aw *AiWorkflowConfig) (*aiflow.CompiledGraph, error) {
	if aw == nil || !aw.Enable {
		return nil, nil
	}
	input := &aiflow.GraphInput{
		Enable:     aw.Enable,
		Entrypoint: aw.Entrypoint,
		Nodes:      make([]aiflow.NodeInput, len(aw.Nodes)),
		Edges:      make([]aiflow.EdgeInput, len(aw.Edges)),
	}
	for i := range aw.Nodes {
		input.Nodes[i] = aiflow.NodeInput{ID: aw.Nodes[i].ID, Type: aw.Nodes[i].Type, Params: aw.Nodes[i].Params}
	}
	for i := range aw.Edges {
		input.Edges[i] = aiflow.EdgeInput{From: aw.Edges[i].From, To: aw.Edges[i].To, Expression: aw.Edges[i].Expression}
	}
	return aiflow.Compile(input)
}

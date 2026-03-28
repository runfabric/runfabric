package config

import (
	"fmt"
	"strings"

	"github.com/runfabric/runfabric/platform/workflow/core/aiflow"
)

// WorkflowCompileConfig holds input for compiling an AI workflow graph.
type WorkflowCompileConfig struct {
	Enable     bool               `yaml:"enable,omitempty"`
	Entrypoint string             `yaml:"entrypoint,omitempty"`
	Nodes      []aiflow.NodeInput `yaml:"nodes,omitempty"`
	Edges      []aiflow.EdgeInput `yaml:"edges,omitempty"`
}

// CompileWorkflowGraph compiles a workflow graph into a hash-stable representation.
func CompileWorkflowGraph(in *WorkflowCompileConfig) (*aiflow.CompiledGraph, error) {
	if in == nil {
		return nil, nil
	}
	return aiflow.Compile(&aiflow.GraphInput{
		Enable:     in.Enable,
		Entrypoint: in.Entrypoint,
		Nodes:      in.Nodes,
		Edges:      in.Edges,
	})
}

// CompileWorkflowGraphFromConfig compiles a graph snapshot from configured workflows.
func CompileWorkflowGraphFromConfig(cfg *Config) (*aiflow.CompiledGraph, error) {
	if cfg == nil {
		return nil, nil
	}

	nodes := make([]aiflow.NodeInput, 0)
	edges := make([]aiflow.EdgeInput, 0)
	entrypoint := ""
	seenNodes := make(map[string]struct{})
	seenEdges := make(map[string]struct{})

	addNode := func(id, typ string, params map[string]any) {
		if strings.TrimSpace(id) == "" {
			return
		}
		if _, ok := seenNodes[id]; ok {
			return
		}
		seenNodes[id] = struct{}{}
		nodes = append(nodes, aiflow.NodeInput{ID: id, Type: typ, Params: params})
	}
	addEdge := func(from, to string) {
		if strings.TrimSpace(from) == "" || strings.TrimSpace(to) == "" {
			return
		}
		key := from + "->" + to
		if _, ok := seenEdges[key]; ok {
			return
		}
		seenEdges[key] = struct{}{}
		edges = append(edges, aiflow.EdgeInput{From: from, To: to})
	}

	for _, wf := range cfg.Workflows {
		wfName := strings.TrimSpace(wf.Name)
		if wfName == "" {
			wfName = "workflow"
		}
		prev := ""
		for i, step := range wf.Steps {
			stepID := strings.TrimSpace(step.ID)
			if stepID == "" {
				stepID = strings.TrimSpace(step.Function)
			}
			if stepID == "" {
				stepID = fmt.Sprintf("step-%d", i+1)
			}
			nodeID := wfName + ":" + stepID
			addNode(nodeID, "code", map[string]any{
				"workflow": wfName,
				"function": step.Function,
				"next":     step.Next,
			})
			if entrypoint == "" {
				entrypoint = nodeID
			}
			if prev != "" {
				addEdge(prev, nodeID)
			}
			prev = nodeID
			if strings.TrimSpace(step.Next) != "" {
				addEdge(nodeID, wfName+":"+strings.TrimSpace(step.Next))
			}
		}
	}

	if len(nodes) == 0 {
		return nil, nil
	}

	return CompileWorkflowGraph(&WorkflowCompileConfig{
		Enable:     true,
		Entrypoint: entrypoint,
		Nodes:      nodes,
		Edges:      edges,
	})
}

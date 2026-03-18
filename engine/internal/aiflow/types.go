package aiflow

// GraphInput is the config-neutral input for Compile. Callers (e.g. config package) convert from config.AiWorkflowConfig.
type GraphInput struct {
	Enable     bool
	Entrypoint string
	Nodes      []NodeInput
	Edges      []EdgeInput
}

// NodeInput is a single node for compilation.
type NodeInput struct {
	ID     string
	Type   string
	Params map[string]any
}

// EdgeInput is a single edge for compilation.
type EdgeInput struct {
	From       string
	To         string
	Expression string
}

// CompiledNode is a node in the compiled DAG (id, type, params). Params are type-specific and not resolved here.
type CompiledNode struct {
	ID     string
	Type   string
	Params map[string]any
}

// CompiledEdge is a resolved edge (from/to node IDs, optional expression). Expressions are not evaluated by the compiler.
type CompiledEdge struct {
	From       string
	To         string
	Expression string
}

// CompiledGraph is the deterministic result of compiling an AI workflow: nodes, edges, topological order, execution levels, and a stable hash.
type CompiledGraph struct {
	Entrypoint string                   // node id that is the workflow entry
	Nodes      map[string]*CompiledNode // id -> node
	Edges      []CompiledEdge           // all edges (sorted for determinism)
	Order      []string                 // topological order (all node IDs)
	Levels     [][]string               // execution tiers from entrypoint; nodes in the same level can run in parallel (async group)
	Hash       string                   // stable hash for plan/deploy/receipts (SHA256 of canonical form)
}

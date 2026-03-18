package aiflow

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// Compile builds a CompiledGraph from GraphInput: resolves edges, detects cycles, computes topological order and execution levels, and a stable hash.
// Returns nil, nil when input is nil or enable is false. Caller must run validation first (unique node IDs, valid types, edge endpoints).
func Compile(input *GraphInput) (*CompiledGraph, error) {
	if input == nil || !input.Enable {
		return nil, nil
	}
	if len(input.Nodes) == 0 {
		return nil, fmt.Errorf("aiWorkflow.nodes is required when enable is true")
	}

	// Build node map (id -> CompiledNode)
	nodes := make(map[string]*CompiledNode)
	for i := range input.Nodes {
		n := &input.Nodes[i]
		id := strings.TrimSpace(n.ID)
		nodes[id] = &CompiledNode{ID: id, Type: strings.TrimSpace(n.Type), Params: n.Params}
	}

	// Build adjacency: successors and predecessors (only edges between known nodes)
	succ := make(map[string][]string)
	pred := make(map[string][]string)
	for _, e := range input.Edges {
		from := strings.TrimSpace(e.From)
		to := strings.TrimSpace(e.To)
		if nodes[from] == nil || nodes[to] == nil {
			continue
		}
		succ[from] = append(succ[from], to)
		pred[to] = append(pred[to], from)
	}

	// Dedupe and sort successors for deterministic order
	for id := range succ {
		succ[id] = sortedUnique(succ[id])
	}

	// Cycle detection via DFS (recursion stack)
	visited := make(map[string]bool)
	stack := make(map[string]bool)
	var cycleStart string
	var dfs func(id string) bool
	dfs = func(id string) bool {
		visited[id] = true
		stack[id] = true
		for _, next := range succ[id] {
			if !visited[next] {
				if dfs(next) {
					return true
				}
			} else if stack[next] {
				cycleStart = next
				return true
			}
		}
		stack[id] = false
		return false
	}
	ids := sortedKeys(nodes)
	for _, id := range ids {
		if !visited[id] && dfs(id) {
			return nil, fmt.Errorf("aiWorkflow cycle detected (involves node %q)", cycleStart)
		}
	}

	// Topological order (Kahn): process nodes with no incoming edges, then remove outgoing edges; use sorted order for determinism
	order := kahnOrder(nodes, pred, succ)
	if len(order) != len(nodes) {
		return nil, fmt.Errorf("aiWorkflow topological order incomplete (cycle?)")
	}

	// Entrypoint and execution levels: level 0 = entrypoint, level k = nodes with max predecessor level k-1
	entrypoint := strings.TrimSpace(input.Entrypoint)
	if entrypoint != "" && nodes[entrypoint] == nil {
		entrypoint = ""
	}
	if entrypoint == "" && len(order) > 0 {
		entrypoint = order[0]
	}
	levels := levelsFromOrder(entrypoint, order, pred, succ)

	// Resolved edges (sorted for determinism)
	edges := make([]CompiledEdge, 0, len(input.Edges))
	for _, e := range input.Edges {
		from := strings.TrimSpace(e.From)
		to := strings.TrimSpace(e.To)
		if nodes[from] != nil && nodes[to] != nil {
			edges = append(edges, CompiledEdge{From: from, To: to, Expression: strings.TrimSpace(e.Expression)})
		}
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		return edges[i].To < edges[j].To
	})

	g := &CompiledGraph{
		Entrypoint: entrypoint,
		Nodes:      nodes,
		Edges:      edges,
		Order:      order,
		Levels:     levels,
	}
	g.Hash = hashGraph(g)
	return g, nil
}

func sortedKeys(m map[string]*CompiledNode) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedUnique(ss []string) []string {
	seen := make(map[string]bool)
	for _, s := range ss {
		seen[s] = true
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func kahnOrder(nodes map[string]*CompiledNode, pred, succ map[string][]string) []string {
	// In-degree per node
	indeg := make(map[string]int)
	for id := range nodes {
		indeg[id] = 0
	}
	for _, to := range succ {
		for _, t := range to {
			indeg[t]++
		}
	}
	var queue []string
	for id := range nodes {
		if indeg[id] == 0 {
			queue = append(queue, id)
		}
	}
	sort.Strings(queue)
	var order []string
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		order = append(order, id)
		for _, next := range succ[id] {
			indeg[next]--
			if indeg[next] == 0 {
				queue = append(queue, next)
				sort.Strings(queue)
			}
		}
	}
	return order
}

// levelsFromOrder returns execution tiers from entrypoint: level 0 = entrypoint, level n = 1+max(pred levels). Only includes nodes reachable from entrypoint.
func levelsFromOrder(entrypoint string, order []string, pred, succ map[string][]string) [][]string {
	if entrypoint == "" || len(order) == 0 {
		return nil
	}
	reachable := reachableFrom(entrypoint, succ)
	levelOf := make(map[string]int)
	levelOf[entrypoint] = 0
	for _, id := range order {
		if id == entrypoint || !reachable[id] {
			continue
		}
		maxPred := -1
		for _, p := range pred[id] {
			if reachable[p] {
				if l := levelOf[p]; l > maxPred {
					maxPred = l
				}
			}
		}
		levelOf[id] = maxPred + 1
	}
	maxL := 0
	for _, l := range levelOf {
		if l > maxL {
			maxL = l
		}
	}
	levels := make([][]string, maxL+1)
	for id, l := range levelOf {
		levels[l] = append(levels[l], id)
	}
	for i := range levels {
		sort.Strings(levels[i])
	}
	return levels
}

func reachableFrom(start string, succ map[string][]string) map[string]bool {
	out := map[string]bool{start: true}
	queue := []string{start}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		for _, next := range succ[id] {
			if !out[next] {
				out[next] = true
				queue = append(queue, next)
			}
		}
	}
	return out
}

// hashGraph returns a stable SHA256 hex hash of the compiled graph (node IDs, types, edges, order).
func hashGraph(g *CompiledGraph) string {
	var b strings.Builder
	ids := make([]string, 0, len(g.Nodes))
	for id := range g.Nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		n := g.Nodes[id]
		b.WriteString("n:")
		b.WriteString(id)
		b.WriteString(":")
		b.WriteString(n.Type)
		b.WriteString("\n")
	}
	for _, e := range g.Edges {
		b.WriteString("e:")
		b.WriteString(e.From)
		b.WriteString("->")
		b.WriteString(e.To)
		b.WriteString("\n")
	}
	for _, id := range g.Order {
		b.WriteString("o:")
		b.WriteString(id)
		b.WriteString("\n")
	}
	b.WriteString("entry:")
	b.WriteString(g.Entrypoint)
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

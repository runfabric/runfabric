package planner

type Node struct {
	ID        string
	DependsOn []string
}

type Graph struct {
	Nodes []Node
}

func NewGraph() *Graph {
	return &Graph{Nodes: []Node{}}
}

func (g *Graph) Add(node Node) {
	g.Nodes = append(g.Nodes, node)
}

package planner

import (
	"testing"
)

func TestNewGraph(t *testing.T) {
	g := NewGraph()
	if g == nil || g.Nodes == nil {
		t.Fatal("NewGraph() returned nil or Nodes nil")
	}
	if len(g.Nodes) != 0 {
		t.Errorf("new graph should have 0 nodes, got %d", len(g.Nodes))
	}
}

func TestGraph_Add(t *testing.T) {
	g := NewGraph()
	g.Add(Node{ID: "a", DependsOn: nil})
	g.Add(Node{ID: "b", DependsOn: []string{"a"}})
	if len(g.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(g.Nodes))
	}
	if g.Nodes[0].ID != "a" || len(g.Nodes[0].DependsOn) != 0 {
		t.Errorf("first node: id=%q DependsOn=%v", g.Nodes[0].ID, g.Nodes[0].DependsOn)
	}
	if g.Nodes[1].ID != "b" || len(g.Nodes[1].DependsOn) != 1 || g.Nodes[1].DependsOn[0] != "a" {
		t.Errorf("second node: id=%q DependsOn=%v", g.Nodes[1].ID, g.Nodes[1].DependsOn)
	}
}

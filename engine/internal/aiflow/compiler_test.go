package aiflow

import (
	"testing"
)

func mkInput(enable bool, entrypoint string, nodes []NodeInput, edges []EdgeInput) *GraphInput {
	return &GraphInput{Enable: enable, Entrypoint: entrypoint, Nodes: nodes, Edges: edges}
}

func TestCompile_NilOrDisabled(t *testing.T) {
	g, err := Compile(nil)
	if err != nil || g != nil {
		t.Fatalf("Compile(nil) want nil,nil got %v, %v", g, err)
	}
	g, err = Compile(&GraphInput{Enable: false})
	if err != nil || g != nil {
		t.Fatalf("Compile(disabled) want nil,nil got %v, %v", g, err)
	}
}

func TestCompile_EmptyNodes(t *testing.T) {
	_, err := Compile(mkInput(true, "", nil, nil))
	if err == nil {
		t.Fatal("expected error when enable and no nodes")
	}
}

func TestCompile_Cycle(t *testing.T) {
	input := mkInput(true, "a",
		[]NodeInput{
			{ID: "a", Type: NodeTypeTrigger},
			{ID: "b", Type: NodeTypeAI},
			{ID: "c", Type: NodeTypeData},
		},
		[]EdgeInput{
			{From: "a", To: "b"},
			{From: "b", To: "c"},
			{From: "c", To: "a"},
		})
	_, err := Compile(input)
	if err == nil {
		t.Fatal("expected error for cycle a->b->c->a")
	}
}

func TestCompile_NoCycle(t *testing.T) {
	input := mkInput(true, "start",
		[]NodeInput{
			{ID: "start", Type: NodeTypeTrigger},
			{ID: "step1", Type: NodeTypeAI},
			{ID: "step2", Type: NodeTypeData},
		},
		[]EdgeInput{
			{From: "start", To: "step1"},
			{From: "step1", To: "step2"},
		})
	g, err := Compile(input)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if g.Entrypoint != "start" {
		t.Errorf("Entrypoint = %q want start", g.Entrypoint)
	}
	if len(g.Order) != 3 {
		t.Errorf("Order len = %d want 3", len(g.Order))
	}
	if len(g.Levels) != 3 {
		t.Errorf("Levels len = %d want 3 (start, step1, step2)", len(g.Levels))
	}
	if g.Hash == "" {
		t.Error("Hash should be set")
	}
	g2, _ := Compile(input)
	if g2.Hash != g.Hash {
		t.Errorf("Hash not deterministic: %q vs %q", g.Hash, g2.Hash)
	}
}

func TestCompile_LevelsReachableOnly(t *testing.T) {
	input := mkInput(true, "a",
		[]NodeInput{
			{ID: "a", Type: NodeTypeTrigger},
			{ID: "b", Type: NodeTypeAI},
			{ID: "orphan", Type: NodeTypeData},
		},
		[]EdgeInput{{From: "a", To: "b"}})
	g, err := Compile(input)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if len(g.Order) != 3 {
		t.Errorf("Order len = %d want 3", len(g.Order))
	}
	if len(g.Levels) != 2 {
		t.Errorf("Levels should have 2 tiers (a, b), got %d", len(g.Levels))
	}
	if len(g.Levels[0]) != 1 || g.Levels[0][0] != "a" {
		t.Errorf("Levels[0] = %v want [a]", g.Levels[0])
	}
	if len(g.Levels[1]) != 1 || g.Levels[1][0] != "b" {
		t.Errorf("Levels[1] = %v want [b]", g.Levels[1])
	}
}

func TestCompile_DeterministicOrderAndHash(t *testing.T) {
	input1 := mkInput(true, "x",
		[]NodeInput{
			{ID: "y", Type: NodeTypeAI},
			{ID: "x", Type: NodeTypeTrigger},
		},
		[]EdgeInput{{From: "x", To: "y"}})
	input2 := mkInput(true, "x",
		[]NodeInput{
			{ID: "x", Type: NodeTypeTrigger},
			{ID: "y", Type: NodeTypeAI},
		},
		[]EdgeInput{{From: "x", To: "y"}})
	g1, _ := Compile(input1)
	g2, _ := Compile(input2)
	if g1.Hash != g2.Hash {
		t.Errorf("Hash should be deterministic: %q vs %q", g1.Hash, g2.Hash)
	}
	if len(g1.Order) != 2 || len(g2.Order) != 2 {
		t.Fatalf("Order length mismatch")
	}
	if g1.Order[0] != "x" || g1.Order[1] != "y" {
		t.Errorf("Order = %v want [x y]", g1.Order)
	}
}

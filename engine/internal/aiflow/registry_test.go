package aiflow

import "testing"

func TestValidNodeType(t *testing.T) {
	for _, typ := range []string{NodeTypeTrigger, NodeTypeAI, NodeTypeData, NodeTypeLogic, NodeTypeSystem, NodeTypeHuman} {
		if !ValidNodeType(typ) {
			t.Errorf("ValidNodeType(%q) = false, want true", typ)
		}
	}
	if ValidNodeType("") {
		t.Error("ValidNodeType(\"\") = true, want false")
	}
	if ValidNodeType("unknown") {
		t.Error("ValidNodeType(\"unknown\") = true, want false")
	}
}

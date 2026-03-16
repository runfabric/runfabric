package cli

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestAddons_List(t *testing.T) {
	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetArgs([]string{"addons", "list"})
	root.SetOut(out)
	root.SetErr(&bytes.Buffer{})
	if err := root.Execute(); err != nil {
		t.Fatalf("addons list should succeed: %v", err)
	}
	if out.Len() == 0 {
		t.Error("addons list should output catalog entries")
	}
	if !bytes.Contains(out.Bytes(), []byte("sentry")) {
		t.Error("addons list should include sentry")
	}
}

func TestAddons_ListJSON(t *testing.T) {
	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetArgs([]string{"addons", "list", "--json"})
	root.SetOut(out)
	root.SetErr(&bytes.Buffer{})
	if err := root.Execute(); err != nil {
		t.Fatalf("addons list --json should succeed: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(out.Bytes(), &m); err != nil {
		t.Fatalf("addons list --json output should be valid JSON: %v", err)
	}
	addons, ok := m["addons"].([]any)
	if !ok || len(addons) == 0 {
		t.Error("addons list --json should have non-empty addons array")
	}
}

package extensions_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	rootcli "github.com/runfabric/runfabric/internal/cli"
)

func TestPlugin_List(t *testing.T) {
	root := rootcli.NewRootCmd()
	out := &bytes.Buffer{}
	root.SetArgs([]string{"extensions", "plugin", "list"})
	root.SetOut(out)
	root.SetErr(&bytes.Buffer{})
	if err := root.Execute(); err != nil {
		t.Fatalf("extensions plugin list should succeed: %v", err)
	}
	if out.Len() == 0 {
		t.Error("extensions plugin list should output at least one plugin name")
	}
}

func TestPlugin_ListJSON(t *testing.T) {
	root := rootcli.NewRootCmd()
	out := &bytes.Buffer{}
	root.SetArgs([]string{"extensions", "plugin", "list", "--json"})
	root.SetOut(out)
	root.SetErr(&bytes.Buffer{})
	if err := root.Execute(); err != nil {
		t.Fatalf("extensions plugin list --json should succeed: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(out.Bytes(), &m); err != nil {
		t.Fatalf("extensions plugin list --json should produce valid JSON: %v", err)
	}
	plugins, ok := m["plugins"].([]any)
	if !ok || len(plugins) == 0 {
		t.Error("extensions plugin list --json should have non-empty plugins array")
	}
}

func TestPlugin_Capabilities_RequiresName(t *testing.T) {
	root := rootcli.NewRootCmd()
	root.SetArgs([]string{"extensions", "plugin", "capabilities"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	err := root.Execute()
	if err == nil {
		t.Fatal("plugin capabilities without name should fail")
	}
	if !strings.Contains(err.Error(), "usage") {
		t.Errorf("expected usage error, got: %v", err)
	}
}

func TestPlugin_Capabilities_NotFound(t *testing.T) {
	root := rootcli.NewRootCmd()
	root.SetArgs([]string{"extensions", "plugin", "capabilities", "nonexistent-provider"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	err := root.Execute()
	if err == nil {
		t.Fatal("plugin capabilities for unknown plugin should fail")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestPlugin_Info_RequiresName(t *testing.T) {
	root := rootcli.NewRootCmd()
	root.SetArgs([]string{"extensions", "plugin", "info"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	err := root.Execute()
	if err == nil {
		t.Fatal("plugin info without name should fail")
	}
	if !strings.Contains(err.Error(), "usage") {
		t.Errorf("expected usage error, got: %v", err)
	}
}

func TestPlugin_Info_NotFound(t *testing.T) {
	root := rootcli.NewRootCmd()
	root.SetArgs([]string{"extensions", "plugin", "info", "nonexistent-provider"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	err := root.Execute()
	if err == nil {
		t.Fatal("plugin info for unknown plugin should fail")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestPlugin_Enable_RequiresName(t *testing.T) {
	root := rootcli.NewRootCmd()
	root.SetArgs([]string{"extensions", "plugin", "enable"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	err := root.Execute()
	if err == nil {
		t.Fatal("plugin enable without name should fail")
	}
	if !strings.Contains(err.Error(), "usage") {
		t.Errorf("expected usage error, got: %v", err)
	}
}

func TestPlugin_Disable_RequiresName(t *testing.T) {
	root := rootcli.NewRootCmd()
	root.SetArgs([]string{"extensions", "plugin", "disable"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	err := root.Execute()
	if err == nil {
		t.Fatal("plugin disable without name should fail")
	}
	if !strings.Contains(err.Error(), "usage") {
		t.Errorf("expected usage error, got: %v", err)
	}
}

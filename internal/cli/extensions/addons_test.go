package extensions_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rootcli "github.com/runfabric/runfabric/internal/cli"
)

func writeTestConfig(t *testing.T, dir string) string {
	t.Helper()
	cfgPath := filepath.Join(dir, "runfabric.yml")
	yml := `service: test
provider:
  name: aws-lambda
  runtime: nodejs20.x
functions:
  - name: api
    entry: src/api.js
`
	if err := os.WriteFile(cfgPath, []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	return cfgPath
}

func TestAddons_List(t *testing.T) {
	root := rootcli.NewRootCmd()
	out := &bytes.Buffer{}
	root.SetArgs([]string{"extensions", "addons", "list"})
	root.SetOut(out)
	root.SetErr(&bytes.Buffer{})
	if err := root.Execute(); err != nil {
		t.Fatalf("extensions addons list should succeed: %v", err)
	}
	if out.Len() == 0 {
		t.Error("extensions addons list should output catalog entries")
	}
	if !bytes.Contains(out.Bytes(), []byte("sentry")) {
		t.Error("extensions addons list should include sentry")
	}
}

func TestAddons_ListJSON(t *testing.T) {
	root := rootcli.NewRootCmd()
	out := &bytes.Buffer{}
	root.SetArgs([]string{"extensions", "addons", "list", "--json"})
	root.SetOut(out)
	root.SetErr(&bytes.Buffer{})
	if err := root.Execute(); err != nil {
		t.Fatalf("extensions addons list --json should succeed: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(out.Bytes(), &m); err != nil {
		t.Fatalf("extensions addons list --json output should be valid JSON: %v", err)
	}
	addons, ok := m["addons"].([]any)
	if !ok || len(addons) == 0 {
		t.Error("extensions addons list --json should have non-empty addons array")
	}
}

func TestAddons_Validate_NoAddonID(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir)

	root := rootcli.NewRootCmd()
	out := &bytes.Buffer{}
	root.SetArgs([]string{"extensions", "addons", "validate", "-c", cfgPath})
	root.SetOut(out)
	root.SetErr(&bytes.Buffer{})
	if err := root.Execute(); err != nil {
		t.Fatalf("addons validate without addon-id should succeed: %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte("ok")) {
		t.Errorf("expected 'ok' in output, got: %s", out.Bytes())
	}
}

func TestAddons_Validate_AddonNotDeclared(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir)

	root := rootcli.NewRootCmd()
	root.SetArgs([]string{"extensions", "addons", "validate", "-c", cfgPath, "sentry"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	err := root.Execute()
	if err == nil {
		t.Fatal("addons validate for undeclared addon should fail")
	}
	if !strings.Contains(err.Error(), "not declared") {
		t.Errorf("expected 'not declared' in error, got: %v", err)
	}
}

func TestAddons_Add_RequiresFunction(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir)

	root := rootcli.NewRootCmd()
	root.SetArgs([]string{"extensions", "addons", "add", "sentry", "-c", cfgPath})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	err := root.Execute()
	if err == nil {
		t.Fatal("addons add without --function should fail")
	}
	if !strings.Contains(err.Error(), "--function is required") {
		t.Errorf("expected '--function is required', got: %v", err)
	}
}

func TestAddons_Add_Success(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir)

	root := rootcli.NewRootCmd()
	out := &bytes.Buffer{}
	root.SetArgs([]string{"extensions", "addons", "add", "sentry", "--function", "api", "-c", cfgPath})
	root.SetOut(out)
	root.SetErr(&bytes.Buffer{})
	if err := root.Execute(); err != nil {
		t.Fatalf("addons add should succeed: %v", err)
	}
	data, _ := os.ReadFile(cfgPath)
	if !strings.Contains(string(data), "sentry") {
		t.Errorf("expected sentry in patched config, got: %s", string(data))
	}
}

func TestAddons_Add_JSON(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir)

	root := rootcli.NewRootCmd()
	out := &bytes.Buffer{}
	root.SetArgs([]string{"extensions", "addons", "add", "datadog", "--function", "api", "-c", cfgPath, "--json"})
	root.SetOut(out)
	root.SetErr(&bytes.Buffer{})
	if err := root.Execute(); err != nil {
		t.Fatalf("addons add --json should succeed: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(out.Bytes(), &m); err != nil {
		t.Fatalf("addons add --json output should be valid JSON: %v", err)
	}
	if m["ok"] != true {
		t.Errorf("expected ok=true in JSON output, got: %v", m)
	}
}

func TestAddons_Remove_RequiresFunction(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir)

	root := rootcli.NewRootCmd()
	root.SetArgs([]string{"extensions", "addons", "remove", "sentry", "-c", cfgPath})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	err := root.Execute()
	if err == nil {
		t.Fatal("addons remove without --function should fail")
	}
	if !strings.Contains(err.Error(), "--function is required") {
		t.Errorf("expected '--function is required', got: %v", err)
	}
}

func TestAddons_Remove_Success(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir)

	// First add the addon
	addRoot := rootcli.NewRootCmd()
	addRoot.SetArgs([]string{"extensions", "addons", "add", "sentry", "--function", "api", "-c", cfgPath})
	addRoot.SetOut(&bytes.Buffer{})
	addRoot.SetErr(&bytes.Buffer{})
	if err := addRoot.Execute(); err != nil {
		t.Fatalf("setup: addons add should succeed: %v", err)
	}

	// Now remove it
	removeRoot := rootcli.NewRootCmd()
	out := &bytes.Buffer{}
	removeRoot.SetArgs([]string{"extensions", "addons", "remove", "sentry", "--function", "api", "-c", cfgPath})
	removeRoot.SetOut(out)
	removeRoot.SetErr(&bytes.Buffer{})
	if err := removeRoot.Execute(); err != nil {
		t.Fatalf("addons remove should succeed: %v", err)
	}

	data, _ := os.ReadFile(cfgPath)
	if strings.Contains(string(data), "sentry") {
		t.Errorf("expected sentry removed from config, got: %s", string(data))
	}
}

func TestAddons_Remove_NoOpWhenNotPresent(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir)

	root := rootcli.NewRootCmd()
	out := &bytes.Buffer{}
	root.SetArgs([]string{"extensions", "addons", "remove", "nonexistent-addon", "--function", "api", "-c", cfgPath})
	root.SetOut(out)
	root.SetErr(&bytes.Buffer{})
	// Should succeed: addon not present is a no-op (function exists)
	if err := root.Execute(); err != nil {
		t.Fatalf("addons remove when addon not present should succeed (no-op): %v", err)
	}
}

func TestAddons_Remove_FunctionNotFound(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir)

	root := rootcli.NewRootCmd()
	root.SetArgs([]string{"extensions", "addons", "remove", "sentry", "--function", "missing-fn", "-c", cfgPath})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	err := root.Execute()
	if err == nil {
		t.Fatal("addons remove for missing function should fail")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestAddons_Remove_JSON(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir)

	// Add first
	addRoot := rootcli.NewRootCmd()
	addRoot.SetArgs([]string{"extensions", "addons", "add", "sentry", "--function", "api", "-c", cfgPath})
	addRoot.SetOut(&bytes.Buffer{})
	addRoot.SetErr(&bytes.Buffer{})
	_ = addRoot.Execute()

	// Remove with --json
	removeRoot := rootcli.NewRootCmd()
	out := &bytes.Buffer{}
	removeRoot.SetArgs([]string{"extensions", "addons", "remove", "sentry", "--function", "api", "-c", cfgPath, "--json"})
	removeRoot.SetOut(out)
	removeRoot.SetErr(&bytes.Buffer{})
	if err := removeRoot.Execute(); err != nil {
		t.Fatalf("addons remove --json should succeed: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(out.Bytes(), &m); err != nil {
		t.Fatalf("addons remove --json output should be valid JSON: %v", err)
	}
	if m["ok"] != true {
		t.Errorf("expected ok=true, got: %v", m)
	}
}

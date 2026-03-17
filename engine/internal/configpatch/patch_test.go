package configpatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestAddFunction_newFunctionsKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runfabric.yml")
	// Minimal config without functions key
	yml := `service: test-svc
provider:
  name: aws-lambda
  runtime: nodejs20.x
`
	if err := os.WriteFile(path, []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := map[string]any{
		"handler": "src/hello.handler",
		"memory":  128,
		"timeout": 10,
		"events":  []any{map[string]any{"http": map[string]any{"method": "get", "path": "/hello"}}},
	}

	err := AddFunction(path, "hello", entry, AddFunctionOptions{Backup: false})
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "hello:") {
		t.Errorf("expected functions.hello in output, got:\n%s", content)
	}
	if !strings.Contains(content, "src/hello.handler") {
		t.Errorf("expected handler path in output, got:\n%s", content)
	}
}

func TestAddFunction_existingFunctions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runfabric.yml")
	yml := `service: test-svc
provider:
  name: aws-lambda
  runtime: nodejs20.x
functions:
  existing:
    handler: src/existing.handler
    memory: 128
`
	if err := os.WriteFile(path, []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := map[string]any{
		"handler": "src/newfn.handler",
		"memory":  128,
		"timeout": 10,
		"events":  []any{map[string]any{"cron": "rate(5 minutes)"}},
	}

	err := AddFunction(path, "newfn", entry, AddFunctionOptions{Backup: false})
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "existing:") || !strings.Contains(content, "newfn:") {
		t.Errorf("expected both existing and newfn in output, got:\n%s", content)
	}
}

func TestAddFunction_collision(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runfabric.yml")
	yml := `service: test-svc
provider:
  name: aws-lambda
  runtime: nodejs20.x
functions:
  hello:
    handler: src/hello.handler
    memory: 128
`
	if err := os.WriteFile(path, []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := map[string]any{
		"handler": "src/other.handler",
		"memory":  128,
		"timeout": 10,
	}

	err := AddFunction(path, "hello", entry, AddFunctionOptions{Backup: false})
	if err == nil {
		t.Fatal("expected error when adding duplicate function name")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' in error, got: %v", err)
	}
}

func TestPlanAddFunction_collision(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runfabric.yml")
	yml := `service: test-svc
provider:
  name: aws-lambda
functions:
  hello:
    handler: src/hello.handler
`
	if err := os.WriteFile(path, []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := map[string]any{"handler": "src/hello.handler"}
	fragment, collision, err := PlanAddFunction(path, "hello", entry)
	if err != nil {
		t.Fatal(err)
	}
	if !collision {
		t.Error("expected collision true when function name already exists")
	}
	if fragment != nil {
		t.Error("expected nil fragment when collision")
	}
}

func TestResolveConfigPath(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runfabric.yml")
	if err := os.WriteFile(cfgPath, []byte("service: x\nprovider:\n  name: aws\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Explicit path
	got, err := ResolveConfigPath(cfgPath, dir, 5)
	if err != nil {
		t.Fatal(err)
	}
	if got != cfgPath {
		t.Errorf("ResolveConfigPath(explicit) = %s, want %s", got, cfgPath)
	}

	// Discover from dir
	got, err = ResolveConfigPath("", dir, 5)
	if err != nil {
		t.Fatal(err)
	}
	if got != cfgPath {
		t.Errorf("ResolveConfigPath(discover) = %s, want %s", got, cfgPath)
	}

	// Not found
	emptyDir := t.TempDir()
	_, err = ResolveConfigPath("", emptyDir, 2)
	if err == nil {
		t.Error("expected error when no config in dir")
	}
}

func TestAddResource(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runfabric.yml")
	if err := os.WriteFile(path, []byte("service: s\nprovider:\n  name: p\n  runtime: r\nfunctions:\n  api:\n    handler: h.js\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	entry := map[string]any{"type": "database", "connectionStringEnv": "DATABASE_URL"}
	err := AddResource(path, "db", entry, AddResourceOptions{Backup: false})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	var root map[string]any
	if err := yaml.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	res, ok := root["resources"].(map[string]any)
	if !ok || res["db"] == nil {
		t.Errorf("resources.db not found: %v", root["resources"])
	}
	// No-op when same entry exists
	err = AddResource(path, "db", entry, AddResourceOptions{Backup: false})
	if err != nil {
		t.Errorf("same entry should be no-op, got %v", err)
	}
	// Error when existing key has different entry
	otherEntry := map[string]any{"type": "cache", "connectionStringEnv": "REDIS_URL"}
	err = AddResource(path, "db", otherEntry, AddResourceOptions{Backup: false})
	if err == nil {
		t.Error("expected error when resource already exists with different entry")
	}
}

func TestAddAddon(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runfabric.yml")
	if err := os.WriteFile(path, []byte("service: s\nprovider:\n  name: p\n  runtime: r\nfunctions:\n  api:\n    handler: h.js\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	entry := map[string]any{"version": "1.0"}
	err := AddAddon(path, "sentry", entry, AddAddonOptions{Backup: false})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	var root map[string]any
	if err := yaml.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	addons, ok := root["addons"].(map[string]any)
	if !ok || addons["sentry"] == nil {
		t.Errorf("addons.sentry not found: %v", root["addons"])
	}
}

func TestAddProviderOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runfabric.yml")
	if err := os.WriteFile(path, []byte("service: s\nprovider:\n  name: p\n  runtime: r\nfunctions:\n  api:\n    handler: h.js\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	entry := map[string]any{"name": "gcp-functions", "runtime": "nodejs20", "region": "us-central1"}
	err := AddProviderOverride(path, "gcp", entry, AddProviderOverrideOptions{Backup: false})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	var root map[string]any
	if err := yaml.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	ov, ok := root["providerOverrides"].(map[string]any)
	if !ok || ov["gcp"] == nil {
		t.Errorf("providerOverrides.gcp not found: %v", root["providerOverrides"])
	}
}

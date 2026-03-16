package workflow

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadCompose_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runfabric.compose.yml")
	content := `services:
  - name: api
    config: ./api/runfabric.yml
  - name: worker
    config: ./worker/runfabric.yml
    dependsOn:
      - api
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := LoadCompose(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(c.Services))
	}
	if c.Services[0].Name != "api" || c.Services[0].Config != "./api/runfabric.yml" {
		t.Errorf("first service: got name=%q config=%q", c.Services[0].Name, c.Services[0].Config)
	}
	if c.Services[1].Name != "worker" || len(c.Services[1].DependsOn) != 1 || c.Services[1].DependsOn[0] != "api" {
		t.Errorf("second service: got name=%q dependsOn=%v", c.Services[1].Name, c.Services[1].DependsOn)
	}
}

func TestLoadCompose_NotFound(t *testing.T) {
	_, err := LoadCompose("/nonexistent/compose.yml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestTopoOrder_NoDeps(t *testing.T) {
	c := &ComposeFile{
		Services: []ComposeService{
			{Name: "a", Config: "./a.yml"},
			{Name: "b", Config: "./b.yml"},
		},
	}
	order, err := TopoOrder(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != 2 {
		t.Fatalf("expected 2, got %d", len(order))
	}
	// Order should contain both
	names := make(map[string]bool)
	for _, n := range order {
		names[n] = true
	}
	if !names["a"] || !names["b"] {
		t.Errorf("order %v missing a or b", order)
	}
}

func TestTopoOrder_WithDeps(t *testing.T) {
	c := &ComposeFile{
		Services: []ComposeService{
			{Name: "worker", Config: "./w.yml", DependsOn: []string{"api"}},
			{Name: "api", Config: "./api.yml"},
		},
	}
	order, err := TopoOrder(c)
	if err != nil {
		t.Fatal(err)
	}
	// api must come before worker
	apiIdx, workerIdx := -1, -1
	for i, n := range order {
		if n == "api" {
			apiIdx = i
		}
		if n == "worker" {
			workerIdx = i
		}
	}
	if apiIdx < 0 || workerIdx < 0 {
		t.Fatalf("order %v missing api or worker", order)
	}
	if apiIdx > workerIdx {
		t.Errorf("api should come before worker, got order %v", order)
	}
}

func TestTopoOrder_UnknownDep(t *testing.T) {
	c := &ComposeFile{
		Services: []ComposeService{
			{Name: "a", Config: "./a.yml", DependsOn: []string{"nonexistent"}},
		},
	}
	_, err := TopoOrder(c)
	if err == nil {
		t.Error("expected error for unknown dependency")
	}
}

func TestResolveServiceConfigPaths(t *testing.T) {
	dir := t.TempDir()
	composePath := filepath.Join(dir, "runfabric.compose.yml")
	_ = os.WriteFile(composePath, []byte("services:\n  - name: s1\n    config: ./c1.yml\n"), 0o600)
	cfgPath := filepath.Join(dir, "c1.yml")
	_ = os.WriteFile(cfgPath, []byte("service: s1\nprovider:\n  name: aws\nfunctions:\n  x: {}\n"), 0o600)

	c, _ := LoadCompose(composePath)
	paths, err := ResolveServiceConfigPaths(composePath, c)
	if err != nil {
		t.Fatal(err)
	}
	if paths["s1"] != cfgPath && paths["s1"] != filepath.Clean(cfgPath) {
		abs, _ := filepath.Abs(cfgPath)
		if paths["s1"] != abs {
			t.Errorf("expected path to c1.yml, got %q", paths["s1"])
		}
	}
}

func TestServiceBindingEnv_EdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		input  map[string]string
		expect map[string]string
	}{
		{"nil", nil, map[string]string{}},
		{"skip empty key", map[string]string{"": "https://x"}, map[string]string{}},
		{"skip empty value", map[string]string{"api": ""}, map[string]string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ServiceBindingEnv(tt.input)
			if !reflect.DeepEqual(got, tt.expect) {
				t.Errorf("ServiceBindingEnv() = %v, want %v", got, tt.expect)
			}
		})
	}
}

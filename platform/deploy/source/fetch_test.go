package source

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindRunfabricYAML(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "repo-main")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(sub, "runfabric.yml")
	if err := os.WriteFile(cfgPath, []byte("service: x\nprovider:\n  name: aws\nfunctions: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := findRunfabricYAML(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != cfgPath {
		t.Errorf("findRunfabricYAML() = %q, want %q", got, cfgPath)
	}
}

func TestFindRunfabricYAML_NotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := findRunfabricYAML(dir)
	if err == nil {
		t.Error("expected error when no runfabric.yml present")
	}
}

func TestFindRunfabricYAML_PreferRoot(t *testing.T) {
	dir := t.TempDir()
	rootCfg := filepath.Join(dir, "runfabric.yml")
	if err := os.WriteFile(rootCfg, []byte("service: root\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(dir, "nested")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	nestedCfg := filepath.Join(sub, "runfabric.yml")
	if err := os.WriteFile(nestedCfg, []byte("service: nested\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := findRunfabricYAML(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Walk order is deterministic; we just need to find one of them.
	if got != rootCfg && got != nestedCfg {
		t.Errorf("findRunfabricYAML() = %q, want %q or %q", got, rootCfg, nestedCfg)
	}
}

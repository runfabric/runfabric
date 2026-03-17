package buildcache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/runfabric/runfabric/engine/internal/config"
)

func TestHashForFunction_MissingFunction(t *testing.T) {
	cfg := &config.Config{
		Service:  "svc",
		Provider: config.ProviderConfig{Runtime: "nodejs20.x"},
		Functions: map[string]config.FunctionConfig{
			"api": {Handler: "src/handler.js"},
		},
	}
	_, err := HashForFunction(cfg, t.TempDir(), "other")
	if err == nil {
		t.Fatal("expected error for missing function")
	}
}

func TestHashForFunction_StableHash(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"p"}`), 0o644)
	cfg := &config.Config{
		Service:  "svc",
		Provider: config.ProviderConfig{Runtime: "nodejs20.x"},
		Functions: map[string]config.FunctionConfig{
			"api": {Handler: "src/handler.js"},
		},
	}
	h1, err := HashForFunction(cfg, dir, "api")
	if err != nil {
		t.Fatal(err)
	}
	h2, err := HashForFunction(cfg, dir, "api")
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Errorf("hash not stable: %s != %s", h1, h2)
	}
}

func TestGetPut(t *testing.T) {
	dir := t.TempDir()
	_, ok := Get(dir, "fn", "abc123")
	if ok {
		t.Fatal("Get should be false when nothing stored")
	}
	if err := Put(dir, "fn", "abc123", "/artifacts/fn.zip"); err != nil {
		t.Fatal(err)
	}
	path, ok := Get(dir, "fn", "abc123")
	if !ok {
		t.Fatal("Get should be true after Put")
	}
	if path != "/artifacts/fn.zip" {
		t.Errorf("got path %q", path)
	}
}

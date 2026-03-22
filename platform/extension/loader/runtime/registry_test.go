package runtime

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/runfabric/runfabric/platform/core/model/config"
)

func TestNormalizeRuntimeID(t *testing.T) {
	cases := map[string]string{
		"nodejs18.x":     "nodejs",
		"nodejs20.x":     "nodejs",
		"runtime-node":   "nodejs",
		"python3.11":     "python",
		"runtime-python": "python",
	}
	for in, want := range cases {
		if got := NormalizeRuntimeID(in); got != want {
			t.Fatalf("NormalizeRuntimeID(%q)=%q want %q", in, got, want)
		}
	}
}

func TestBuiltinRegistry_NodeBuild(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "handler.js"), []byte("exports.handler = async ()=>({ok:true})\n"), 0o644); err != nil {
		t.Fatalf("write handler: %v", err)
	}

	reg := NewBuiltinRegistry()
	rt, err := reg.Get("nodejs20.x")
	if err != nil {
		t.Fatalf("get runtime: %v", err)
	}
	artifact, err := rt.Build(context.Background(), BuildRequest{
		Root:            root,
		FunctionName:    "api",
		FunctionConfig:  config.FunctionConfig{Handler: "src/handler.handler", Runtime: "nodejs20.x"},
		ConfigSignature: "sig",
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if artifact == nil || artifact.OutputPath == "" {
		t.Fatal("expected artifact output path")
	}
	if _, err := os.Stat(artifact.OutputPath); err != nil {
		t.Fatalf("expected output zip: %v", err)
	}
}

func TestBuiltinRegistry_PythonBuild(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "handler.py"), []byte("def handler(event, context):\n    return {'ok': True}\n"), 0o644); err != nil {
		t.Fatalf("write handler: %v", err)
	}

	reg := NewBuiltinRegistry()
	rt, err := reg.Get("python3.11")
	if err != nil {
		t.Fatalf("get runtime: %v", err)
	}
	artifact, err := rt.Build(context.Background(), BuildRequest{
		Root:            root,
		FunctionName:    "api",
		FunctionConfig:  config.FunctionConfig{Handler: "src/handler.handler", Runtime: "python3.11"},
		ConfigSignature: "sig",
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if artifact == nil || artifact.OutputPath == "" {
		t.Fatal("expected artifact output path")
	}
	if _, err := os.Stat(artifact.OutputPath); err != nil {
		t.Fatalf("expected output zip: %v", err)
	}
}

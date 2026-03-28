package runtimes_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/runfabric/runfabric/platform/core/model/config"
	"github.com/runfabric/runfabric/platform/extensions/registry/resolution"
)

func TestNormalizeRuntimeID(t *testing.T) {
	cases := map[string]string{
		"nodejs18.x": "nodejs",
		"nodejs20.x": "nodejs",
		"python3.11": "python",
	}
	for in, want := range cases {
		b, err := resolution.NewCached(resolution.Options{IncludeExternal: false})
		if err != nil {
			t.Fatalf("new boundary: %v", err)
		}
		m, err := b.ResolveRuntime(in)
		if err != nil {
			t.Fatalf("resolve runtime %q: %v", in, err)
		}
		if got := m.ID; got != want {
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

	b, err := resolution.NewCached(resolution.Options{IncludeExternal: false})
	if err != nil {
		t.Fatalf("new boundary: %v", err)
	}
	artifact, err := b.BuildFunction(t.Context(), resolution.RuntimeBuildRequest{
		Runtime:         "nodejs20.x",
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

	b, err := resolution.NewCached(resolution.Options{IncludeExternal: false})
	if err != nil {
		t.Fatalf("new boundary: %v", err)
	}
	artifact, err := b.BuildFunction(t.Context(), resolution.RuntimeBuildRequest{
		Runtime:         "python3.11",
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

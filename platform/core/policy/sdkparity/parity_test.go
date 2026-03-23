package sdkparity

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCrossSDKHandlerParityMarkers(t *testing.T) {
	root := repoRoot(t)
	cases := []struct {
		path     string
		contains []string
	}{
		{
			path: filepath.Join(root, "packages", "node", "sdk", "src", "index.d.ts"),
			contains: []string{
				"createHandler",
				"LifecycleHook",
				"runLifecycleHooks",
			},
		},
		{
			path: filepath.Join(root, "packages", "python", "runfabric", "src", "runfabric", "__init__.py"),
			contains: []string{
				"create_asgi_handler",
				"create_wsgi_handler",
			},
		},
		{
			path: filepath.Join(root, "packages", "go", "sdk", "handler", "handler.go"),
			contains: []string{
				"type Handler",
				"type Context",
			},
		},
		{
			path: filepath.Join(root, "packages", "java", "sdk", "src", "main", "java", "dev", "runfabric", "Handler.java"),
			contains: []string{
				"interface Handler",
				"handle(",
			},
		},
		{
			path: filepath.Join(root, "packages", "dotnet", "sdk", "RunFabric.Sdk", "Handler.cs"),
			contains: []string{
				"delegate",
				"HandlerContext",
			},
		},
	}

	for _, tc := range cases {
		b, err := os.ReadFile(tc.path)
		if err != nil {
			t.Fatalf("read %s: %v", tc.path, err)
		}
		content := string(b)
		for _, marker := range tc.contains {
			if !strings.Contains(content, marker) {
				t.Fatalf("%s missing marker %q", tc.path, marker)
			}
		}
	}
}

func TestHookParityDocsAligned(t *testing.T) {
	root := repoRoot(t)
	docPath := filepath.Join(root, "docs", "SDK_FRAMEWORKS.md")
	b, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("read %s: %v", docPath, err)
	}
	content := string(b)
	for _, marker := range []string{
		"Hook parity",
		"Node SDK",
		"hooks",
	} {
		if !strings.Contains(content, marker) {
			t.Fatalf("%s missing marker %q", docPath, marker)
		}
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	// repoRoot is discovered instead of hard-coded path traversal so the test keeps working
	// after on-disk repo re-orgs (e.g. moving `engine/` under `platform/`).
	dir := filepath.Dir(file)
	for i := 0; i < 10; i++ {
		if isDir(filepath.Join(dir, "packages")) && isDir(filepath.Join(dir, "docs")) {
			return filepath.Clean(dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("repo root not found starting from %s", filepath.Dir(file))
	return ""
}

func isDir(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}

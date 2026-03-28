package cmd_test

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"testing"
)

func TestBinaryEntrypointImportsAreIsolated(t *testing.T) {
	tests := []struct {
		name   string
		file   string
		must   []string
		forbid []string
	}{
		{
			name: "runfabric",
			file: filepath.Join("runfabric", "main.go"),
			must: []string{
				"github.com/runfabric/runfabric/internal/cli",
			},
			forbid: []string{
				"github.com/runfabric/runfabric/internal/cli/daemon",
				"github.com/runfabric/runfabric/internal/cli/daemoncmd",
				"github.com/runfabric/runfabric/internal/cli/worker",
			},
		},
		{
			name: "runfabricd",
			file: filepath.Join("runfabricd", "main.go"),
			must: []string{
				"github.com/runfabric/runfabric/internal/cli/daemon",
			},
			forbid: []string{
				"github.com/runfabric/runfabric/internal/cli",
				"github.com/runfabric/runfabric/internal/cli/worker",
			},
		},
		{
			name: "runfabricw",
			file: filepath.Join("runfabricw", "main.go"),
			must: []string{
				"github.com/runfabric/runfabric/internal/cli/worker",
			},
			forbid: []string{
				"github.com/runfabric/runfabric/internal/cli",
				"github.com/runfabric/runfabric/internal/cli/daemon",
				"github.com/runfabric/runfabric/internal/cli/daemoncmd",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			imports := fileImports(t, tc.file)
			for _, m := range tc.must {
				if !imports[m] {
					t.Fatalf("expected %s to import %q", tc.file, m)
				}
			}
			for _, f := range tc.forbid {
				if imports[f] {
					t.Fatalf("expected %s not to import %q", tc.file, f)
				}
			}
		})
	}
}

func fileImports(t *testing.T, relPath string) map[string]bool {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, relPath, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse %s: %v", relPath, err)
	}
	imports := map[string]bool{}
	for _, imp := range f.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			t.Fatalf("unquote import path %s: %v", imp.Path.Value, err)
		}
		imports[path] = true
	}
	return imports
}

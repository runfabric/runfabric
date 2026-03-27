package architecture

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

const modulePrefix = "github.com/runfabric/runfabric/"

var aliasOnlyTypePattern = regexp.MustCompile(`(?m)^\s*(type\s+[A-Za-z0-9_]+\s*=|[A-Za-z0-9_]+\s*=)`)

func TestImportGraphConstraints(t *testing.T) {
	root := repoRoot(t)
	files := goFiles(t, root)

	var violations []string
	var platformExtensionImporters []string

	for _, rel := range files {
		imports := fileImports(t, filepath.Join(root, rel))
		for _, importPath := range imports {
			if isPlatformExtensionsRootImport(rel, importPath) {
				platformExtensionImporters = append(platformExtensionImporters, rel)
			}
			if ok, reason := importAllowed(rel, importPath); !ok {
				violations = append(violations, rel+": "+importPath+" ("+reason+")")
			}
		}
	}

	sort.Strings(platformExtensionImporters)
	platformExtensionImporters = compact(platformExtensionImporters)
	if len(platformExtensionImporters) != 1 || platformExtensionImporters[0] != "platform/extensions/providerpolicy/providers.go" {
		violations = append(violations, "expected exactly one platform/extensions root importer: platform/extensions/providerpolicy/providers.go; found "+strings.Join(platformExtensionImporters, ", "))
	}

	if len(violations) > 0 {
		t.Fatalf("architecture import violations:\n%s", strings.Join(violations, "\n"))
	}
}

func TestGoldenImportRulePatterns(t *testing.T) {
	cases := []struct {
		name       string
		file       string
		importPath string
		allowed    bool
	}{
		{
			name:       "extensions cannot import internal",
			file:       "extensions/providers/aws/example.go",
			importPath: modulePrefix + "internal/provider/contracts",
			allowed:    false,
		},
		{
			name:       "extensions cannot import platform",
			file:       "extensions/runtimes/example.go",
			importPath: modulePrefix + "platform/extensions/providerpolicy",
			allowed:    false,
		},
		{
			name:       "internal cannot import extensions",
			file:       "internal/app/example.go",
			importPath: modulePrefix + "extensions/routers",
			allowed:    false,
		},
		{
			name:       "packages cannot import platform",
			file:       "packages/go/plugin-sdk/router/types.go",
			importPath: modulePrefix + "platform/core/model/config",
			allowed:    false,
		},
		{
			name:       "providerpolicy is the allowed platform importer",
			file:       "platform/extensions/providerpolicy/providers.go",
			importPath: modulePrefix + "extensions/routers",
			allowed:    true,
		},
		{
			name:       "other platform extensions files cannot import root extensions",
			file:       "platform/extensions/registry/resolution/builtins.go",
			importPath: modulePrefix + "extensions/runtimes",
			allowed:    false,
		},
		{
			name:       "extensions may import plugin sdk",
			file:       "extensions/routers/registry.go",
			importPath: modulePrefix + "plugin-sdk/go/router",
			allowed:    true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ok, _ := importAllowed(tc.file, tc.importPath)
			if ok != tc.allowed {
				t.Fatalf("file=%s import=%s allowed=%v want=%v", tc.file, tc.importPath, ok, tc.allowed)
			}
		})
	}
}

func TestOwnershipADRAndInvariants(t *testing.T) {
	root := repoRoot(t)

	adrPath := filepath.Join(root, "docs", "ARCHITECTURE_OWNERSHIP.md")
	adrBytes, err := os.ReadFile(adrPath)
	if err != nil {
		t.Fatalf("read %s: %v", adrPath, err)
	}
	adr := string(adrBytes)
	for _, marker := range []string{
		"Status: Accepted",
		"Canonical ownership table",
		"platform/extensions/providerpolicy/providers.go",
		"Forbidden edges",
		"router",
		"runtime",
		"simulator",
		"provider",
	} {
		if !strings.Contains(adr, marker) {
			t.Fatalf("%s missing marker %q", adrPath, marker)
		}
	}

	for _, rel := range []string{
		"internal/extensions/contracts/types.go",
		"internal/extensions/routers/registry.go",
		"internal/extensions/routers/cloudflare.go",
		"internal/extensions/runtimes/registry.go",
		"internal/extensions/simulators/simulators.go",
		"internal/extensions/builtins/loaders.go",
	} {
		path := filepath.Join(root, rel)
		if _, err := os.Stat(path); err == nil {
			t.Fatalf("duplicate internal extension shim remains: %s", path)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat %s: %v", path, err)
		}
	}
}

func importAllowed(relPath, importPath string) (bool, string) {
	relPath = filepath.ToSlash(relPath)

	if strings.HasPrefix(relPath, "extensions/") {
		if strings.HasPrefix(importPath, modulePrefix+"internal/") {
			return false, "Rule 1: extensions must not import internal"
		}
		if strings.HasPrefix(importPath, modulePrefix+"platform/") {
			return false, "Rule 1: extensions must not import platform"
		}
	}

	if strings.HasPrefix(relPath, "internal/") && strings.HasPrefix(importPath, modulePrefix+"extensions/") {
		return false, "Flow: internal must not import extensions"
	}

	if (strings.HasPrefix(relPath, "packages/") || strings.HasPrefix(relPath, "platform/extensions/external/testdata/")) && strings.HasPrefix(importPath, modulePrefix+"platform/") {
		return false, "Legacy boundary: packages and testdata must not import platform"
	}

	if strings.HasPrefix(relPath, "platform/extensions/") && relPath != "platform/extensions/providerpolicy/providers.go" && strings.HasPrefix(importPath, modulePrefix+"extensions/") {
		return false, "Rule 2b: only providerpolicy/providers.go may import root extensions"
	}

	return true, ""
}

func isPlatformExtensionsRootImport(relPath, importPath string) bool {
	relPath = filepath.ToSlash(relPath)
	return strings.HasPrefix(relPath, "platform/extensions/") && strings.HasPrefix(importPath, modulePrefix+"extensions/")
}

func fileImports(t *testing.T, path string) []string {
	t.Helper()
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse imports %s: %v", path, err)
	}
	imports := make([]string, 0, len(parsed.Imports))
	for _, imp := range parsed.Imports {
		imports = append(imports, strings.Trim(imp.Path.Value, `"`))
	}
	return imports
}

func goFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		if d.IsDir() {
			switch rel {
			case ".git", "bin", ".venv":
				return filepath.SkipDir
			}
			return nil
		}

		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
			return nil
		}
		files = append(files, rel)
		return nil
	})
	if err != nil {
		t.Fatalf("walk repo: %v", err)
	}
	return files
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(file)
	for i := 0; i < 10; i++ {
		if isDir(filepath.Join(dir, "docs")) && isDir(filepath.Join(dir, "platform")) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("repo root not found from %s", file)
	return ""
}

func isDir(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}

func compact(values []string) []string {
	if len(values) == 0 {
		return values
	}
	out := values[:0]
	var prev string
	for _, value := range values {
		if len(out) == 0 || value != prev {
			out = append(out, value)
			prev = value
		}
	}
	return out
}

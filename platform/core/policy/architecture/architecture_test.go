package architecture

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

const modulePrefix = "github.com/runfabric/runfabric/"

var aliasOnlyTypePattern = regexp.MustCompile(`(?m)^\s*type\s+[A-Za-z0-9_]+\s*=`)
var bannedTermPattern = regexp.MustCompile(`(?i)\b(bridge|alias|canonical|facade|wrapper|wrapping)\b`)
var bannedIdentifierTerms = map[string]struct{}{
	"bridge":    {},
	"alias":     {},
	"canonical": {},
	"facade":    {},
	"wrapper":   {},
	"wrapping":  {},
}

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
	for _, imp := range platformExtensionImporters {
		if !strings.HasPrefix(imp, "platform/extensions/providerpolicy/") {
			violations = append(violations, "expected root extension importers to be in platform/extensions/providerpolicy/; found "+imp)
		}
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
			name:       "extensions cannot import provider contracts",
			file:       "extensions/providers/aws/example.go",
			importPath: modulePrefix + "platform/core/contracts/provider",
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
			name:       "internal cannot import plugin sdk directly",
			file:       "internal/cli/router/router.go",
			importPath: modulePrefix + "plugin-sdk/go/router",
			allowed:    false,
		},
		{
			name:       "cli must not import workflow lifecycle directly",
			file:       "internal/cli/extensions/plugin.go",
			importPath: modulePrefix + "platform/workflow/lifecycle",
			allowed:    false,
		},
		{
			name:       "cli must not import workflow recovery directly",
			file:       "internal/cli/lifecycle/recover.go",
			importPath: modulePrefix + "platform/workflow/recovery",
			allowed:    false,
		},
		{
			name:       "cli must not import deploy source directly",
			file:       "internal/cli/lifecycle/deploy.go",
			importPath: modulePrefix + "platform/deploy/source",
			allowed:    false,
		},
		{
			name:       "cli may import workflow app boundary",
			file:       "internal/cli/lifecycle/deploy.go",
			importPath: modulePrefix + "platform/workflow/app",
			allowed:    true,
		},
		{
			name:       "plugin sdk cannot import platform",
			file:       "packages/go/plugin-sdk/router/types.go",
			importPath: modulePrefix + "platform/core/model/config",
			allowed:    false,
		},
		{
			name:       "plugin sdk cannot import provider contracts",
			file:       "packages/go/plugin-sdk/router/types.go",
			importPath: modulePrefix + "platform/core/contracts/provider",
			allowed:    false,
		},
		{
			name:       "platform cannot import packages directly",
			file:       "platform/workflow/app/example.go",
			importPath: modulePrefix + "packages/go/plugin-sdk/runtime",
			allowed:    false,
		},
		{
			name:       "platform cannot import plugin sdk directly",
			file:       "platform/workflow/app/router_sync.go",
			importPath: modulePrefix + "plugin-sdk/go/router",
			allowed:    false,
		},
		{
			name:       "plugin sdk may import root extensions",
			file:       "packages/go/plugin-sdk/router/types.go",
			importPath: modulePrefix + "extensions/routers",
			allowed:    true,
		},
		{
			name:       "non plugin-sdk packages cannot import root extensions",
			file:       "packages/node/sdk/index.go",
			importPath: modulePrefix + "extensions/routers",
			allowed:    false,
		},
		{
			name:       "providerpolicy is the allowed platform importer",
			file:       "platform/extensions/providerpolicy/providers.go",
			importPath: modulePrefix + "extensions/routers",
			allowed:    true,
		},
		{
			name:       "providerpolicy builtin files may import root extensions",
			file:       "platform/extensions/providerpolicy/builtin_states.go",
			importPath: modulePrefix + "extensions/states/dynamodb",
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

	for _, rel := range []string{
		"platform/core/planner/engine",
		"platform/core/state/locking",
		"platform/runtime/build",
		"platform/deploy/runtime",
		"platform/deploy/testinghooks",
	} {
		path := filepath.Join(root, rel)
		if _, err := os.Stat(path); err == nil {
			t.Fatalf("duplicate package mirror remains: %s", path)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat %s: %v", path, err)
		}
	}
}

func TestRule4NoAliasBridgeArtifacts(t *testing.T) {
	root := repoRoot(t)

	// These paths previously hosted alias bridge layers and must remain direct/canonical.
	noAliasFiles := []string{
		"extensions/routers/registry.go",
		"platform/extensions/providerpolicy/providers.go",
	}

	for _, rel := range noAliasFiles {
		path := filepath.Join(root, rel)
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if aliasOnlyTypePattern.Match(body) {
			t.Fatalf("rule4 violation: alias/re-export type detected in %s", rel)
		}
	}

	// Historic alias-only bridge file must stay deleted.
	removedBridge := filepath.Join(root, "platform/extensions/registry/resolution/types_aliases.go")
	if _, err := os.Stat(removedBridge); err == nil {
		t.Fatalf("rule4 violation: deleted alias bridge file restored: %s", removedBridge)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", removedBridge, err)
	}

	removedAppFacade := filepath.Join(root, "internal/app/app.go")
	if _, err := os.Stat(removedAppFacade); err == nil {
		t.Fatalf("rule4 violation: deleted app facade restored: %s", removedAppFacade)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", removedAppFacade, err)
	}

	for _, rel := range []string{
		"apps/sdkbridge",
		"internal/provider/sdkbridge",
	} {
		path := filepath.Join(root, rel)
		if _, err := os.Stat(path); err == nil {
			t.Fatalf("rule4 violation: bridge/facade package remains: %s", path)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat %s: %v", path, err)
		}
	}
}

func TestRule4NoTypeAliasesRepoWide(t *testing.T) {
	root := repoRoot(t)
	files := goFiles(t, root)

	var violations []string
	for _, rel := range files {
		body, err := readFileWithTimeout(filepath.Join(root, rel), 2*time.Second)
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		if aliasOnlyTypePattern.Match(body) {
			violations = append(violations, rel)
		}
	}

	if len(violations) > 0 {
		sort.Strings(violations)
		t.Fatalf("rule4 violation: type aliases are not allowed; found in:\n%s", strings.Join(violations, "\n"))
	}
}

func TestRule4NoBannedTermsInCodeText(t *testing.T) {
	root := repoRoot(t)
	files := goFiles(t, root)
	fset := token.NewFileSet()

	var violations []string
	for _, rel := range files {
		path := filepath.Join(root, rel)
		src, err := readFileWithTimeout(path, 2*time.Second)
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		parsed, err := parser.ParseFile(fset, path, src, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", rel, err)
		}

		if _, found := bannedIdentifierTerms[strings.ToLower(parsed.Name.Name)]; found {
			pos := fset.Position(parsed.Name.Pos())
			violations = append(violations, rel+":"+itoa(pos.Line)+": package name contains banned term ("+parsed.Name.Name+")")
		}

		for _, cg := range parsed.Comments {
			for _, c := range cg.List {
				if term := bannedTermPattern.FindString(c.Text); term != "" {
					pos := fset.Position(c.Pos())
					violations = append(violations, rel+":"+itoa(pos.Line)+": comment contains banned term ("+strings.ToLower(term)+")")
				}
			}
		}

		ast.Inspect(parsed, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if !ok || id == nil || id.Name == "_" {
				return true
			}
			if _, found := bannedIdentifierTerms[strings.ToLower(id.Name)]; !found {
				return true
			}
			pos := fset.Position(id.Pos())
			violations = append(violations, rel+":"+itoa(pos.Line)+": identifier uses banned term ("+id.Name+")")
			return true
		})
	}

	if len(violations) > 0 {
		sort.Strings(violations)
		t.Fatalf("rule4 violation: banned terms found in package/comments/identifiers:\n%s", strings.Join(violations, "\n"))
	}
}

func itoa(v int) string {
	return strconv.Itoa(v)
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

	if strings.HasPrefix(relPath, "internal/") && strings.HasPrefix(importPath, modulePrefix+"plugin-sdk/") {
		return false, "Rule 2h: internal must not import plugin-sdk directly"
	}

	if strings.HasPrefix(relPath, "internal/cli/") {
		if importPath == modulePrefix+"platform/workflow/lifecycle" {
			return false, "Rule 3a: internal/cli must route lifecycle operations via platform/workflow/app"
		}
		if importPath == modulePrefix+"platform/workflow/recovery" {
			return false, "Rule 3a: internal/cli must route recovery operations via platform/workflow/app"
		}
		if importPath == modulePrefix+"platform/deploy/source" {
			return false, "Rule 3a: internal/cli must route source deploy operations via platform/workflow/app"
		}
	}

	if strings.HasPrefix(relPath, "packages/go/plugin-sdk/") && strings.HasPrefix(importPath, modulePrefix+"platform/") {
		return false, "Rule 2c: plugin-sdk must not import platform directly"
	}

	if strings.HasPrefix(relPath, "packages/go/plugin-sdk/") && strings.HasPrefix(importPath, modulePrefix+"internal/") {
		return false, "Rule 2d: plugin-sdk must not import internal directly"
	}

	if strings.HasPrefix(relPath, "packages/") && strings.HasPrefix(importPath, modulePrefix+"platform/") {
		return false, "Rule 2e: packages must not import platform directly"
	}

	if strings.HasPrefix(relPath, "packages/") && !strings.HasPrefix(relPath, "packages/go/plugin-sdk/") && strings.HasPrefix(importPath, modulePrefix+"extensions/") {
		return false, "Rule 2g: only plugin-sdk may import root extensions"
	}

	if strings.HasPrefix(relPath, "platform/extensions/application/external/testdata/") && strings.HasPrefix(importPath, modulePrefix+"platform/") {
		return false, "Rule 2j: external plugin testdata stubs must not import platform"
	}

	if strings.HasPrefix(relPath, "platform/") && strings.HasPrefix(importPath, modulePrefix+"packages/") {
		return false, "Rule 2f: platform must not import packages directly"
	}

	if strings.HasPrefix(relPath, "platform/") && strings.HasPrefix(importPath, modulePrefix+"plugin-sdk/") {
		return false, "Rule 2i: platform must not import plugin-sdk directly"
	}

	if strings.HasPrefix(relPath, "platform/extensions/") && !strings.HasPrefix(relPath, "platform/extensions/providerpolicy/") && strings.HasPrefix(importPath, modulePrefix+"extensions/") {
		return false, "Rule 2b: only providerpolicy may import root extensions"
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
	src, err := readFileWithTimeout(path, 2*time.Second)
	if err != nil {
		t.Fatalf("read imports %s: %v", path, err)
	}
	parsed, err := parser.ParseFile(fset, path, src, parser.ImportsOnly)
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

func readFileWithTimeout(path string, timeout time.Duration) ([]byte, error) {
	type result struct {
		body []byte
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		body, err := os.ReadFile(path)
		ch <- result{body: body, err: err}
	}()

	select {
	case r := <-ch:
		return r.body, r.err
	case <-time.After(timeout):
		return nil, fmt.Errorf("read timeout after %s", timeout)
	}
}

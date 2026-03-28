package unit

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/runfabric/runfabric/platform/core/model/config"
)

var legacyTopLevelConfigKeyPattern = regexp.MustCompile(`(?m)^(runtime|entry|providers|triggers):\s*`)

func TestExampleConfigsUseCanonicalShape(t *testing.T) {
	root := repoRoot(t)
	patterns := []string{
		"examples/node/hello-http/runfabric*.yml",
		"examples/node/compose-contracts/*/runfabric.yml",
		"examples/node/handler-scenarios/*/runfabric.yml",
	}

	var files []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(root, pattern))
		if err != nil {
			t.Fatalf("glob %q: %v", pattern, err)
		}
		files = append(files, matches...)
	}
	if len(files) == 0 {
		t.Fatal("no example config files found")
	}

	for _, file := range files {
		base := filepath.Base(file)
		if strings.Contains(base, ".compose.") {
			continue
		}

		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		if legacyTopLevelConfigKeyPattern.Match(data) {
			t.Fatalf("%s contains legacy top-level keys (runtime/entry/providers/triggers)", file)
		}

		cfg, err := config.Load(file)
		if err != nil {
			t.Fatalf("config.Load(%s): %v", file, err)
		}
		if err := config.Validate(cfg); err != nil {
			t.Fatalf("config.Validate(%s): %v", file, err)
		}
	}
}

func TestKeyDocsDoNotReintroduceLegacyTopLevelShape(t *testing.T) {
	root := repoRoot(t)
	for _, rel := range []string{
		"docs/QUICKSTART.md",
		"docs/HANDLER_SCENARIOS.md",
	} {
		path := filepath.Join(root, rel)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		if legacyTopLevelConfigKeyPattern.Match(data) {
			t.Fatalf("%s contains legacy top-level keys (runtime/entry/providers/triggers)", rel)
		}
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := wd
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		next := filepath.Dir(dir)
		if next == dir {
			break
		}
		dir = next
	}
	t.Fatal("could not locate repo root (go.mod)")
	return ""
}

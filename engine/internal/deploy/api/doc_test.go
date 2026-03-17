package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAPIProviderNames_Wired asserts that every provider returned by APIProviderNames
// is wired (HasRunner) and that the list is non-empty.
func TestAPIProviderNames_Wired(t *testing.T) {
	names := APIProviderNames()
	if len(names) == 0 {
		t.Fatal("APIProviderNames() should return at least one provider")
	}
	for _, name := range names {
		if !HasRunner(name) {
			t.Errorf("provider %q in APIProviderNames() but HasRunner(%q) is false", name, name)
		}
	}
}

// TestAPIProviderNames_DocSync asserts that DEPLOY_PROVIDERS.md (when present)
// mentions each API provider so the doc stays in sync with code.
func TestAPIProviderNames_DocSync(t *testing.T) {
	var data []byte
	var err error
	// Try paths: from repo root (docs/), from engine (../docs), from engine/internal/deploy/api (../../../../docs).
	for _, base := range []string{"docs", filepath.Join("..", "docs"), filepath.Join("..", "..", "..", "..", "docs")} {
		docPath := filepath.Join(base, "DEPLOY_PROVIDERS.md")
		data, err = os.ReadFile(docPath)
		if err == nil {
			break
		}
	}
	if err != nil {
		t.Skipf("DEPLOY_PROVIDERS.md not found (run from repo root or engine): %v", err)
	}
	doc := string(data)
	for _, name := range APIProviderNames() {
		if !strings.Contains(doc, name) {
			t.Errorf("DEPLOY_PROVIDERS.md should mention provider %q (add it so doc stays in sync with deploy API)", name)
		}
	}
}

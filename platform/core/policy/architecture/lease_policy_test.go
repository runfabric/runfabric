package architecture

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The plugin state contract is a deliberate copy: extensions/... may not
// import internal/..., so extensions/types/types.go duplicates
// internal/state/types/types.go. The copy is only safe while it is identical —
// this test turns silent divergence into a build failure.
func TestStateContractCopyStaysIdentical(t *testing.T) {
	root := repoRoot(t)

	engineSide, err := os.ReadFile(filepath.Join(root, "internal/state/types/types.go"))
	if err != nil {
		t.Fatalf("read engine-side contract: %v", err)
	}
	pluginSide, err := os.ReadFile(filepath.Join(root, "extensions/types/types.go"))
	if err != nil {
		t.Fatalf("read plugin-side contract: %v", err)
	}

	if !bytes.Equal(engineSide, pluginSide) {
		t.Fatal("lease/lock contract copies diverged: internal/state/types/types.go and extensions/types/types.go must stay byte-identical (edit both in the same change)")
	}
}

// Lease renewal loops have exactly two homes: internal/lease (engine side) and
// packages/go/plugin-sdk/lease (plugin side of the extension boundary). Any
// other file combining a ticker with renewal logic is a reintroduced duplicate
// of the lease primitive.
func TestLeaseHeartbeatHasSingleHome(t *testing.T) {
	root := repoRoot(t)
	files := goFiles(t, root)

	allowed := map[string]struct{}{
		"internal/lease/lease.go":               {},
		"packages/go/plugin-sdk/lease/lease.go": {},
	}
	renewPattern := regexp.MustCompile(`(?i)renew`)

	var violations []string
	for _, rel := range files {
		if strings.HasSuffix(rel, "_test.go") {
			continue
		}
		if _, ok := allowed[rel]; ok {
			continue
		}
		body, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		if bytes.Contains(body, []byte("time.NewTicker")) && renewPattern.Match(body) {
			violations = append(violations, rel)
		}
	}

	if len(violations) > 0 {
		t.Fatalf("lease heartbeat loops outside internal/lease and plugin-sdk/lease (use those primitives instead):\n%s", strings.Join(violations, "\n"))
	}
}

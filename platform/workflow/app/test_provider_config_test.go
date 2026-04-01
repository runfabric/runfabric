package app

import (
	"strings"
	"testing"

	manifests "github.com/runfabric/runfabric/platform/extensions/manifest"
	"github.com/runfabric/runfabric/platform/extensions/registry/resolution"
)

func testProviderNameAndRuntime(t *testing.T) (string, string) {
	t.Helper()
	boundary, err := resolution.New(resolution.Options{IncludeExternal: false})
	if err == nil {
		providers := boundary.PluginRegistry().List(manifests.KindProvider)
		for _, p := range providers {
			if p == nil {
				continue
			}
			if id := strings.TrimSpace(p.ID); id != "" {
				return id, "nodejs20.x"
			}
		}
	}
	// Fallback to a common built-in provider/runtime pair used by existing tests.
	return "aws-lambda", "nodejs20.x"
}

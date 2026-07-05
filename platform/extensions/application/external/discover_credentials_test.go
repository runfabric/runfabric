package external

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeRawPlugin writes a plugin dir with a literal plugin.yaml (for shapes
// the typed pluginYAML helper cannot express, e.g. malformed credentials).
func writeRawPlugin(t *testing.T, dir, yamlBody, executable string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if executable != "" {
		if err := os.WriteFile(filepath.Join(dir, executable), []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.yaml"), []byte(yamlBody), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDiscover_BadCredentialsMarkedInvalid(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(envHome, tmp)

	writeRawPlugin(t, filepath.Join(tmp, "plugins", "routers", "bad-creds", "0.1.0"), `
apiVersion: runfabric.io/plugin/v1
kind: router
id: bad-creds
name: Bad Creds Router
version: 0.1.0
executable: ./bad-router
capabilities:
  - sync
credentials:
  - envKey: lower_case_key
    header: X-Ok-Header
`, "bad-router")

	res, err := Discover(DiscoverOptions{IncludeInvalid: true})
	if err != nil {
		t.Fatalf("Discover error: %v", err)
	}
	if len(res.Plugins) != 0 {
		t.Fatalf("plugin with malformed credentials must be rejected, got %#v", res.Plugins)
	}
	if len(res.Invalid) == 0 || !strings.Contains(res.Invalid[0].Reason, "plugin.yaml credentials") {
		t.Fatalf("expected credentials validation reason, got %#v", res.Invalid)
	}
}

func TestDiscover_ValidCredentialsAccepted(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(envHome, tmp)

	writeRawPlugin(t, filepath.Join(tmp, "plugins", "routers", "good-creds", "0.1.0"), `
apiVersion: runfabric.io/plugin/v1
kind: router
id: good-creds
name: Good Creds Router
version: 0.1.0
executable: ./good-router
capabilities:
  - sync
credentials:
  - envKey: MY_ROUTER_TOKEN
    header: X-My-Router-Token
    required: true
`, "good-router")

	res, err := Discover(DiscoverOptions{IncludeInvalid: true})
	if err != nil {
		t.Fatalf("Discover error: %v", err)
	}
	if len(res.Plugins) != 1 {
		t.Fatalf("valid plugin must load, invalid=%#v", res.Invalid)
	}
	creds := res.Plugins[0].Credentials
	if len(creds) != 1 || creds[0].EnvKey != "MY_ROUTER_TOKEN" || creds[0].Header != "X-My-Router-Token" || !creds[0].Required {
		t.Fatalf("credentials not carried into manifest: %#v", creds)
	}
}

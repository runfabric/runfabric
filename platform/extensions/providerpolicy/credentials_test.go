package providerpolicy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// hostableProviders are the built-ins that take per-request credentials via
// daemon headers; kubernetes is env/file-only and declares no headers.
var hostableProviders = []string{
	"aws-lambda", "gcp-functions", "azure-functions", "alibaba-fc",
	"cloudflare-workers", "digitalocean-functions", "fly-machines",
	"ibm-openwhisk", "netlify", "vercel",
}

func TestBuiltinProviderCredentialDeclarations(t *testing.T) {
	seenHeaders := map[string]string{}
	for _, id := range append(append([]string{}, hostableProviders...), "kubernetes") {
		creds := ProviderCredentials(id)
		if len(creds) == 0 {
			t.Errorf("%s: no credential declaration", id)
			continue
		}
		required := 0
		for _, c := range creds {
			if c.EnvKey == "" {
				t.Errorf("%s: credential with empty EnvKey", id)
			}
			if c.Required {
				required++
			}
			if c.Header == "" {
				continue
			}
			if !strings.HasPrefix(c.Header, "X-Provider-") {
				t.Errorf("%s: header %q must start with X-Provider-", id, c.Header)
			}
			if prev, dup := seenHeaders[c.Header]; dup {
				t.Errorf("header %q declared by both %s and %s", c.Header, prev, id)
			}
			seenHeaders[c.Header] = id
		}
		if required == 0 {
			t.Errorf("%s: no required credential declared", id)
		}
	}
	for _, id := range hostableProviders {
		hasHeader := false
		for _, c := range ProviderCredentials(id) {
			if c.Header != "" {
				hasHeader = true
			}
		}
		if !hasHeader {
			t.Errorf("%s: hostable provider declares no daemon headers", id)
		}
	}
	for _, c := range ProviderCredentials("kubernetes") {
		if c.Header != "" {
			t.Errorf("kubernetes: %s must stay env-only (kubeconfig/registry creds are not per-request)", c.EnvKey)
		}
	}
}

func TestStateBackendCredentialDeclarations(t *testing.T) {
	// Backends with real credentials must declare them; embedded/local ones
	// must not. State credentials are env-only — never daemon headers.
	for kind, wantRequired := range map[string][]string{
		"postgres": {"RUNFABRIC_STATE_POSTGRES_URL"},
		"s3":       {"RUNFABRIC_S3_BUCKET"},
		"dynamodb": {"RUNFABRIC_DYNAMODB_TABLE"},
		"gcs":      {"RUNFABRIC_GCS_BUCKET"},
		"azblob":   {"RUNFABRIC_AZBLOB_CONTAINER"},
	} {
		creds := StateBackendCredentials(kind)
		if len(creds) == 0 {
			t.Errorf("%s: no state credential declaration", kind)
			continue
		}
		var required []string
		for _, c := range creds {
			// State secrets may ride per-request X-State-* headers; anything
			// else (manifest-carried names, cross-covered vars) stays env-only.
			if c.Header != "" && !strings.HasPrefix(c.Header, "X-State-") {
				t.Errorf("%s: state header %q must start with X-State-", kind, c.Header)
			}
			if c.Required {
				required = append(required, c.EnvKey)
			}
		}
		if strings.Join(required, ",") != strings.Join(wantRequired, ",") {
			t.Errorf("%s: required vars = %v, want %v", kind, required, wantRequired)
		}
	}
	// The BYO-state secrets must be per-request capable.
	for kind, wantHeaders := range map[string]int{"postgres": 1, "azblob": 3} {
		got := 0
		for _, c := range StateBackendCredentials(kind) {
			if c.Header != "" {
				got++
			}
		}
		if got != wantHeaders {
			t.Errorf("%s: expected %d per-request headers, got %d", kind, wantHeaders, got)
		}
	}
	for _, kind := range []string{"local", "sqlite", "unknown"} {
		if creds := StateBackendCredentials(kind); len(creds) != 0 {
			t.Errorf("%s: expected no credential declaration, got %v", kind, creds)
		}
	}
	// Headers must be globally unique across providers AND state backends.
	seen := map[string]string{}
	for _, id := range hostableProviders {
		for _, c := range ProviderCredentials(id) {
			if c.Header != "" {
				seen[c.Header] = id
			}
		}
	}
	for kind, creds := range AllStateBackendCredentials() {
		for _, c := range creds {
			if c.Header == "" {
				continue
			}
			if prev, dup := seen[c.Header]; dup {
				t.Errorf("header %q declared by both %s and state backend %s", c.Header, prev, kind)
			}
			seen[c.Header] = "state:" + kind
		}
	}
}

// TestPluginYamlCredentialsMatchDeclarations pins plugin.yaml (the external-
// plugin declaration) to CredentialVars (the built-in declaration) so the two
// sources cannot drift.
func TestPluginYamlCredentialsMatchDeclarations(t *testing.T) {
	type yamlCred struct {
		EnvKey      string `yaml:"envKey"`
		Header      string `yaml:"header"`
		Required    bool   `yaml:"required"`
		Mirror      string `yaml:"mirror"`
		Placeholder string `yaml:"placeholder"`
	}
	var manifest struct {
		Credentials []yamlCred `yaml:"credentials"`
	}
	for _, id := range append(append([]string{}, hostableProviders...), "kubernetes") {
		path := filepath.Join("..", "..", "..", "extensions", "providers", id, "plugin.yaml")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: read plugin.yaml: %v", id, err)
		}
		manifest.Credentials = nil
		if err := yaml.Unmarshal(data, &manifest); err != nil {
			t.Fatalf("%s: parse plugin.yaml: %v", id, err)
		}
		declared := ProviderCredentials(id)
		if len(manifest.Credentials) != len(declared) {
			t.Errorf("%s: plugin.yaml declares %d credentials, code declares %d", id, len(manifest.Credentials), len(declared))
			continue
		}
		for i, c := range declared {
			y := manifest.Credentials[i]
			if y.EnvKey != c.EnvKey || y.Header != c.Header || y.Required != c.Required || y.Mirror != c.Mirror || y.Placeholder != c.Placeholder {
				t.Errorf("%s: credential %d differs: plugin.yaml=%+v code=%+v", id, i, y, c)
			}
		}
	}
}

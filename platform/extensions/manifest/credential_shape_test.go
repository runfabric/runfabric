package manifests_test

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	manifests "github.com/runfabric/runfabric/platform/extensions/manifest"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy/catalog"
)

// Every built-in credential declaration — all extension kinds — must satisfy
// the same shape rules enforced on external plugin.yaml at load time
// (manifests.ValidateCredentialSpecs). One contract, both origins.
// Lives in manifests_test (not providerpolicy) to avoid an import cycle:
// manifests imports providerpolicy.
func TestAllBuiltinCredentialDeclarationsAreWellFormed(t *testing.T) {
	check := func(origin string, creds []catalog.CredentialVar) {
		t.Helper()
		if err := manifests.ValidateCredentialSpecs(manifests.CredentialSpecs(creds)); err != nil {
			t.Errorf("%s: %v", origin, err)
		}
	}

	for _, d := range providerpolicy.All() {
		check("provider:"+d.ID, d.Credentials)
	}
	for kind, creds := range providerpolicy.AllStateBackendCredentials() {
		check("state:"+kind, creds)
	}
	check("stateAws", providerpolicy.StateAWSCredentialVars())
	check("router", providerpolicy.RouterCredentialVars())
	for id, creds := range providerpolicy.RouterPluginCredentialVars() {
		check("routerPlugin:"+id, creds)
	}
	for id, creds := range providerpolicy.SecretManagerCredentialVars() {
		check("secretManager:"+id, creds)
	}
}

// Every plugin.yaml under extensions/ — any kind, any module (including
// external-only plugins like linode that have no in-engine Go declaration) —
// must carry credentials that pass the same shape rules enforced at plugin
// load. A filesystem walk, not a plugin list: extensions move external over
// time and this sweep must keep covering whatever is in the tree.
func TestAllInTreePluginYamlCredentialsAreWellFormed(t *testing.T) {
	root := filepath.Join("..", "..", "..", "extensions")
	found := 0
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "plugin.yaml" {
			return err
		}
		found++
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Errorf("read %s: %v", path, readErr)
			return nil
		}
		var m struct {
			ID          string `yaml:"id"`
			Credentials []struct {
				EnvKey      string `yaml:"envKey"`
				Header      string `yaml:"header"`
				Required    bool   `yaml:"required"`
				Mirror      string `yaml:"mirror"`
				Placeholder string `yaml:"placeholder"`
				Fallback    string `yaml:"fallback"`
			} `yaml:"credentials"`
		}
		if err := yaml.Unmarshal(data, &m); err != nil {
			t.Errorf("parse %s: %v", path, err)
			return nil
		}
		specs := make([]manifests.CredentialSpec, 0, len(m.Credentials))
		for _, c := range m.Credentials {
			specs = append(specs, manifests.CredentialSpec{
				EnvKey: c.EnvKey, Header: c.Header, Required: c.Required,
				Mirror: c.Mirror, Placeholder: c.Placeholder, Fallback: c.Fallback,
			})
		}
		if err := manifests.ValidateCredentialSpecs(specs); err != nil {
			t.Errorf("%s (%s): %v", path, m.ID, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if found < 20 {
		t.Fatalf("walk found only %d plugin.yaml files — extensions tree moved?", found)
	}
}

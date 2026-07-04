package providerpolicy

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/runfabric/runfabric/platform/extensions/providerpolicy/catalog"
)

// credentialsJSONPath is the credential contract shipped with the Node daemon
// client so downstream platforms (e.g. runfabric-paas) can validate their
// catalogs against the framework's declarations instead of duplicating them.
const credentialsJSONPath = "../../../packages/node/daemon-client/src/credentials.json"

type credentialJSON struct {
	EnvKey      string `json:"envKey"`
	Header      string `json:"header,omitempty"`
	Required    bool   `json:"required,omitempty"`
	Mirror      string `json:"mirror,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
}

func credentialContract() ([]byte, error) {
	toJSON := func(creds []catalog.CredentialVar) []credentialJSON {
		out := make([]credentialJSON, 0, len(creds))
		for _, c := range creds {
			out = append(out, credentialJSON{
				EnvKey: c.EnvKey, Header: c.Header, Required: c.Required,
				Mirror: c.Mirror, Placeholder: c.Placeholder,
			})
		}
		return out
	}
	providers := map[string][]credentialJSON{}
	for _, d := range All() {
		if len(d.Credentials) > 0 {
			providers[d.ID] = toJSON(d.Credentials)
		}
	}
	state := map[string][]credentialJSON{}
	kinds := make([]string, 0)
	for kind, creds := range AllStateBackendCredentials() {
		kinds = append(kinds, kind)
		state[kind] = toJSON(creds)
	}
	sort.Strings(kinds)
	data, err := json.MarshalIndent(map[string]any{"providers": providers, "state": state}, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// TestDaemonClientCredentialsContractInSync pins packages/node/daemon-client's
// credentials.json to the Go declarations. Regenerate with:
//
//	UPDATE_CREDENTIALS_JSON=1 go test ./platform/extensions/providerpolicy -run DaemonClientCredentials
func TestDaemonClientCredentialsContractInSync(t *testing.T) {
	want, err := credentialContract()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Clean(credentialsJSONPath)
	if os.Getenv("UPDATE_CREDENTIALS_JSON") == "1" {
		if err := os.WriteFile(path, want, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", path)
		return
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v (regenerate with UPDATE_CREDENTIALS_JSON=1)", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s is stale — regenerate with:\n  UPDATE_CREDENTIALS_JSON=1 go test ./platform/extensions/providerpolicy -run DaemonClientCredentials", path)
	}
}

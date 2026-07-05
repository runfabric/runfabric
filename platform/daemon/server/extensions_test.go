package server

import (
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/runfabric/runfabric/platform/extensions/application/external"
	manifests "github.com/runfabric/runfabric/platform/extensions/manifest"
	"github.com/runfabric/runfabric/platform/extensions/registry/resolution"
)

func TestHandleExtensionsListsKindsAndBuiltins(t *testing.T) {
	// Point plugin discovery at an empty home so only built-ins are listed.
	t.Setenv("RUNFABRIC_HOME", t.TempDir())

	rec := httptest.NewRecorder()
	handleExtensions(rec, httptest.NewRequest("GET", "/extensions", nil))

	if rec.Code != 200 {
		t.Fatalf("status = %d, body: %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Kinds []struct {
			Kind      string `json:"kind"`
			ConfigKey string `json:"configKey"`
			Default   string `json:"default"`
			Plugins   []struct {
				ID          string                     `json:"id"`
				Source      string                     `json:"source"`
				Credentials []manifests.CredentialSpec `json:"credentials"`
			} `json:"plugins"`
		} `json:"kinds"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	byKind := map[string]struct {
		configKey string
		def       string
		ids       map[string]string
	}{}
	for _, k := range payload.Kinds {
		ids := map[string]string{}
		for _, p := range k.Plugins {
			ids[p.ID] = p.Source
		}
		byKind[k.Kind] = struct {
			configKey string
			def       string
			ids       map[string]string
		}{k.ConfigKey, k.Default, ids}
	}

	cases := []struct {
		kind      string
		configKey string
		wantID    string
	}{
		{"provider", "provider.name", "aws-lambda"},
		{"router", "extensions.routerPlugin", "cloudflare"},
		{"secret-manager", "extensions.secretManagerPlugin", "vault-secret-manager"},
		{"state", "extensions.statePlugin", "postgres"},
	}
	for _, c := range cases {
		got, ok := byKind[c.kind]
		if !ok {
			t.Fatalf("kind %q missing from response", c.kind)
		}
		if got.configKey != c.configKey {
			t.Errorf("kind %q configKey = %q, want %q", c.kind, got.configKey, c.configKey)
		}
		if src, ok := got.ids[c.wantID]; !ok {
			t.Errorf("kind %q missing builtin plugin %q (have %v)", c.kind, c.wantID, got.ids)
		} else if src != "builtin" {
			t.Errorf("plugin %q source = %q, want builtin", c.wantID, src)
		}
	}
	if byKind["router"].def != "cloudflare" {
		t.Errorf("router default = %q, want cloudflare", byKind["router"].def)
	}
	// Runtime and simulator mappings are present even when no plugins are installed.
	for _, kind := range []string{"runtime", "simulator"} {
		if _, ok := byKind[kind]; !ok {
			t.Errorf("kind %q missing from response", kind)
		}
	}

	// Credential declarations (incl. declarative fallbacks) ship in the
	// payload for EVERY plugin of every kind — asserted against the registry
	// manifests, never against specific plugin IDs: extensions can move
	// external and the engine (and this test) must stay dynamic.
	catalog, err := resolution.DiscoverPluginCatalog(external.DiscoverOptions{})
	if err != nil || catalog == nil || catalog.Registry == nil {
		t.Fatalf("catalog: %v", err)
	}
	declared := map[string][]manifests.CredentialSpec{}
	declaredWithCreds := 0
	for _, m := range catalog.Registry.List("") {
		declared[string(m.Kind)+"/"+m.ID] = m.Credentials
		if len(m.Credentials) > 0 {
			declaredWithCreds++
		}
	}
	served := 0
	for _, k := range payload.Kinds {
		for _, p := range k.Plugins {
			want := declared[k.Kind+"/"+p.ID]
			if !reflect.DeepEqual(p.Credentials, want) {
				t.Errorf("%s/%s: payload credentials %+v != declared %+v", k.Kind, p.ID, p.Credentials, want)
			}
			if len(p.Credentials) > 0 {
				served++
			}
		}
	}
	if declaredWithCreds == 0 || served != declaredWithCreds {
		t.Errorf("credentialed plugins served = %d, declared = %d — payload must mirror the registry", served, declaredWithCreds)
	}
}

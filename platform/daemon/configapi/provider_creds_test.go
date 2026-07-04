package configapi

import (
	"net/http"
	"sort"
	"testing"
)

func TestCollectProviderCredsAws(t *testing.T) {
	h := http.Header{}
	h.Set("X-Provider-Aws-Access-Key-Id", "AKIA123")
	h.Set("X-Provider-Aws-Region", "ap-south-1")

	creds, touched := collectProviderCreds(h)

	if creds["AWS_ACCESS_KEY_ID"] != "AKIA123" || creds["AWS_REGION"] != "ap-south-1" {
		t.Fatalf("unexpected creds: %v", creds)
	}
	if creds["AWS_DEFAULT_REGION"] != "ap-south-1" {
		t.Fatalf("AWS region must mirror to AWS_DEFAULT_REGION, got %v", creds)
	}
	// The FULL AWS group is touched (clean slate), even keys without a header value.
	want := []string{"AWS_ACCESS_KEY_ID", "AWS_DEFAULT_REGION", "AWS_REGION", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN"}
	sort.Strings(touched)
	if len(touched) != len(want) {
		t.Fatalf("touched = %v, want %v", touched, want)
	}
	for i, k := range want {
		if touched[i] != k {
			t.Fatalf("touched = %v, want %v", touched, want)
		}
	}
}

func TestCollectProviderCredsNonAwsProviders(t *testing.T) {
	cases := []struct {
		header string
		value  string
		envKey string
	}{
		{"X-Provider-Gcp-Access-Token", "ya29.token", "GCP_ACCESS_TOKEN"},
		{"X-Provider-Cloudflare-Api-Token", "cf-token", "CLOUDFLARE_API_TOKEN"},
		{"X-Provider-Azure-Access-Token", "az-token", "AZURE_ACCESS_TOKEN"},
		{"X-Provider-Vercel-Token", "vc-token", "VERCEL_TOKEN"},
		{"X-Provider-Netlify-Auth-Token", "nl-token", "NETLIFY_AUTH_TOKEN"},
		{"X-Provider-Digitalocean-Access-Token", "do-token", "DIGITALOCEAN_ACCESS_TOKEN"},
		{"X-Provider-Alibaba-Access-Key-Id", "ali-key", "ALIBABA_ACCESS_KEY_ID"},
		{"X-Provider-Fly-Api-Token", "fly-token", "FLY_API_TOKEN"},
		{"X-Provider-Ibm-Auth", "ibm-auth", "IBM_OPENWHISK_AUTH"},
	}
	for _, tc := range cases {
		h := http.Header{}
		h.Set(tc.header, tc.value)
		creds, touched := collectProviderCreds(h)
		if creds[tc.envKey] != tc.value {
			t.Errorf("%s: expected %s=%s, got %v", tc.header, tc.envKey, tc.value, creds)
		}
		if len(touched) == 0 {
			t.Errorf("%s: expected the provider group to be touched", tc.header)
		}
		// A single provider's headers must not activate other groups.
		for _, k := range touched {
			if k == "AWS_ACCESS_KEY_ID" && tc.envKey != "AWS_ACCESS_KEY_ID" {
				t.Errorf("%s: AWS group must not be touched", tc.header)
			}
		}
	}
}

func TestCollectProviderCredsGcpProjectMirrors(t *testing.T) {
	h := http.Header{}
	h.Set("X-Provider-Gcp-Project", "my-proj")
	creds, _ := collectProviderCreds(h)
	if creds["GCP_PROJECT"] != "my-proj" || creds["GCP_PROJECT_ID"] != "my-proj" {
		t.Fatalf("GCP project must mirror to both spellings, got %v", creds)
	}
}

func TestCollectProviderCredsEmpty(t *testing.T) {
	creds, touched := collectProviderCreds(http.Header{})
	if len(creds) != 0 || len(touched) != 0 {
		t.Fatalf("expected no creds without headers, got %v / %v", creds, touched)
	}
}

// State-backend secrets ride X-State-* headers so a tenant can bring their own
// state store (postgres DSN, azure storage auth) in the same request as the
// provider's X-Provider-* credentials.
func TestCollectStateCreds(t *testing.T) {
	h := http.Header{}
	h.Set("X-Provider-Aws-Access-Key-Id", "AKIA")
	h.Set("X-State-Postgres-Url", "postgres://u:p@db:5432/state")
	creds, touched := collectProviderCreds(h)
	if creds["AWS_ACCESS_KEY_ID"] != "AKIA" {
		t.Errorf("provider group must apply alongside state group, got %v", creds)
	}
	if creds["RUNFABRIC_STATE_POSTGRES_URL"] != "postgres://u:p@db:5432/state" {
		t.Errorf("postgres state header not mapped, got %v", creds)
	}
	touchedSet := map[string]bool{}
	for _, k := range touched {
		touchedSet[k] = true
	}
	if !touchedSet["RUNFABRIC_STATE_POSTGRES_URL"] || !touchedSet["AWS_ACCESS_KEY_ID"] {
		t.Errorf("both groups must be touched, got %v", touched)
	}
}

func TestCollectStateCredsAzblobFullGroupTouch(t *testing.T) {
	// One azblob header present → the FULL azblob group is cleared, so a
	// partial per-request set never mixes with ambient azure storage creds.
	h := http.Header{}
	h.Set("X-State-Azblob-Connection-String", "BlobEndpoint=http://x;AccountName=a;AccountKey=k")
	creds, touched := collectProviderCreds(h)
	if creds["AZURE_STORAGE_CONNECTION_STRING"] == "" {
		t.Fatalf("connection string not mapped, got %v", creds)
	}
	touchedSet := map[string]bool{}
	for _, k := range touched {
		touchedSet[k] = true
	}
	for _, k := range []string{"AZURE_STORAGE_CONNECTION_STRING", "AZURE_STORAGE_ACCOUNT", "AZURE_STORAGE_KEY"} {
		if !touchedSet[k] {
			t.Errorf("expected %s in touched azblob group, got %v", k, touched)
		}
	}
}

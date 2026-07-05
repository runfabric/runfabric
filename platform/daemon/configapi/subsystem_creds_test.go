package configapi

import (
	"net/http"
	"os"
	"sort"
	"testing"
	"time"
)

func TestCollectRouterCreds(t *testing.T) {
	h := http.Header{}
	h.Set("X-Router-Api-Token", "cf-token-123")
	h.Set("X-Router-Zone-Id", "zone-9")

	creds, touched := collectProviderCreds(h)

	if creds["RUNFABRIC_ROUTER_API_TOKEN"] != "cf-token-123" {
		t.Fatalf("router token not mapped: %v", creds)
	}
	if creds["RUNFABRIC_ROUTER_ZONE_ID"] != "zone-9" {
		t.Fatalf("zone id not mapped: %v", creds)
	}
	// The FULL router group is cleared (clean slate), including the account id
	// that was not sent — ambient daemon creds must never mix with a partial set.
	want := []string{"RUNFABRIC_ROUTER_ACCOUNT_ID", "RUNFABRIC_ROUTER_API_TOKEN", "RUNFABRIC_ROUTER_ZONE_ID"}
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

func TestCollectVaultSecretManagerCreds(t *testing.T) {
	h := http.Header{}
	h.Set("X-Secret-Vault-Token", "hvs.abc")
	h.Set("X-Secret-Vault-Addr", "https://vault.internal:8200")

	creds, touched := collectProviderCreds(h)

	if creds["VAULT_TOKEN"] != "hvs.abc" || creds["VAULT_ADDR"] != "https://vault.internal:8200" {
		t.Fatalf("vault creds not mapped: %v", creds)
	}
	found := false
	for _, k := range touched {
		if k == "VAULT_NAMESPACE" {
			found = true
		}
	}
	if !found {
		t.Fatalf("VAULT_NAMESPACE must be cleared with the group, touched = %v", touched)
	}
}

func TestRouterCredsDoNotActivateOtherGroups(t *testing.T) {
	h := http.Header{}
	h.Set("X-Router-Api-Token", "tok")
	_, touched := collectProviderCreds(h)
	for _, k := range touched {
		if k == "AWS_ACCESS_KEY_ID" || k == "VAULT_TOKEN" {
			t.Fatalf("router headers must not touch other groups, touched %v", touched)
		}
	}
}

func TestWithProviderCredsAppliesAndRestoresRouterEnv(t *testing.T) {
	t.Setenv("RUNFABRIC_ROUTER_API_TOKEN", "ambient-token")
	t.Setenv("RUNFABRIC_ROUTER_ACCOUNT_ID", "ambient-account")

	s := NewServer("dev")
	req, _ := http.NewRequest("POST", "/deploy", nil)
	req.Header.Set("X-Router-Api-Token", "request-token")

	var insideToken, insideAccount string
	err := s.withProviderCreds(req, func() error {
		insideToken = os.Getenv("RUNFABRIC_ROUTER_API_TOKEN")
		insideAccount = os.Getenv("RUNFABRIC_ROUTER_ACCOUNT_ID")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if insideToken != "request-token" {
		t.Fatalf("inside fn token = %q, want request-token", insideToken)
	}
	// Account id was NOT sent → the group clear must hide the ambient value.
	if insideAccount != "" {
		t.Fatalf("inside fn account = %q, want empty (group cleared)", insideAccount)
	}
	if got := os.Getenv("RUNFABRIC_ROUTER_API_TOKEN"); got != "ambient-token" {
		t.Fatalf("after fn token = %q, ambient value must be restored", got)
	}
	if got := os.Getenv("RUNFABRIC_ROUTER_ACCOUNT_ID"); got != "ambient-account" {
		t.Fatalf("after fn account = %q, ambient value must be restored", got)
	}
}

func TestWithProviderCredsSerializesWithoutHeaders(t *testing.T) {
	s := NewServer("dev")
	req, _ := http.NewRequest("POST", "/deploy", nil)

	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	go func() {
		_ = s.withProviderCreds(req, func() error {
			close(entered)
			<-release
			return nil
		})
		close(done)
	}()
	<-entered

	// A second headerless op must block until the first releases deployMu.
	second := make(chan struct{})
	go func() {
		_ = s.withProviderCreds(req, func() error { return nil })
		close(second)
	}()
	// Give the second goroutine time to run — it must be parked on deployMu.
	time.Sleep(50 * time.Millisecond)
	select {
	case <-second:
		t.Fatal("second op ran concurrently; deploy/remove must serialize even without credential headers")
	default:
	}
	close(release)
	<-done
	<-second
}

// The three AWS identities (deploy target, state store, secret source) must be
// independently addressable and must never clear each other's env keys.
func TestThreeAWSIdentitiesAreIsolated(t *testing.T) {
	h := http.Header{}
	h.Set("X-Provider-Aws-Access-Key-Id", "AKIA_DEPLOY")
	h.Set("X-Provider-Aws-Secret-Access-Key", "deploy-secret")
	h.Set("X-State-Aws-Access-Key-Id", "AKIA_STATE")
	h.Set("X-State-Aws-Secret-Access-Key", "state-secret")
	h.Set("X-Secret-Aws-Access-Key-Id", "AKIA_SM")
	h.Set("X-Secret-Aws-Secret-Access-Key", "sm-secret")

	creds, _ := collectProviderCreds(h)

	if creds["AWS_ACCESS_KEY_ID"] != "AKIA_DEPLOY" {
		t.Fatalf("deploy identity: %v", creds)
	}
	if creds["RUNFABRIC_STATE_AWS_ACCESS_KEY_ID"] != "AKIA_STATE" {
		t.Fatalf("state identity: %v", creds)
	}
	if creds["RUNFABRIC_SM_AWS_ACCESS_KEY_ID"] != "AKIA_SM" {
		t.Fatalf("secret-source identity: %v", creds)
	}
}

// Sending ONLY a scoped state identity must not clear the deploy identity's
// AWS_* keys (distinct env-key sets → distinct groups).
func TestStateAwsGroupDoesNotTouchProviderAwsKeys(t *testing.T) {
	h := http.Header{}
	h.Set("X-State-Aws-Access-Key-Id", "AKIA_STATE")
	h.Set("X-State-Aws-Secret-Access-Key", "state-secret")
	_, touched := collectProviderCreds(h)
	for _, k := range touched {
		if k == "AWS_ACCESS_KEY_ID" || k == "AWS_SECRET_ACCESS_KEY" {
			t.Fatalf("state group must not touch provider AWS keys, touched %v", touched)
		}
	}
}

// Multi-cloud PaaS deploys forward several provider groups in ONE request
// (e.g. AWS deploy target + cloudflare creds for the router's declarative
// fallback). Every group whose headers are present must apply independently.
func TestTwoProviderGroupsApplyInOneRequest(t *testing.T) {
	h := http.Header{}
	h.Set("X-Provider-Aws-Access-Key-Id", "AKIA_DEPLOY")
	h.Set("X-Provider-Aws-Secret-Access-Key", "deploy-secret")
	h.Set("X-Provider-Cloudflare-Api-Token", "cf-tenant-token")

	creds, touched := collectProviderCreds(h)

	if creds["AWS_ACCESS_KEY_ID"] != "AKIA_DEPLOY" {
		t.Fatalf("aws group not applied: %v", creds)
	}
	if creds["CLOUDFLARE_API_TOKEN"] != "cf-tenant-token" {
		t.Fatalf("cloudflare group not applied alongside aws: %v", creds)
	}
	// Both groups' full env sets are cleared; unrelated groups untouched.
	sawCF, sawAWS := false, false
	for _, k := range touched {
		switch k {
		case "CLOUDFLARE_ACCOUNT_ID":
			sawCF = true
		case "AWS_SESSION_TOKEN":
			sawAWS = true
		case "VAULT_TOKEN", "RUNFABRIC_ROUTER_API_TOKEN":
			t.Fatalf("unrelated group touched: %v", touched)
		}
	}
	if !sawCF || !sawAWS {
		t.Fatalf("both groups' full key sets must be cleared, touched %v", touched)
	}
}

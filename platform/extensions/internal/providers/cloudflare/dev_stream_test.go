package cloudflare

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/runfabric/runfabric/platform/core/model/config"
	"github.com/runfabric/runfabric/platform/deploy/apiutil"
)

func TestRedirectToTunnel_RewritesAndRestoresRoutes(t *testing.T) {
	var (
		proxyUploaded bool
		routeToProxy  bool
		routeRestored bool
		proxyDeleted  bool
	)

	workerName := "demo-dev"
	proxyName := workerName + "-devstream-proxy"
	routeID := "route-1"
	zoneID := "zone-123"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/client/v4/zones/"+zoneID+"/workers/routes":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success":true,"result":[{"id":"` + routeID + `","pattern":"example.com/*","script":"` + workerName + `"}]}`))
			return
		case r.Method == http.MethodPut && r.URL.Path == "/client/v4/accounts/account-123/workers/scripts/"+proxyName:
			proxyUploaded = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"success":true}`))
			return
		case r.Method == http.MethodPut && r.URL.Path == "/client/v4/zones/"+zoneID+"/workers/routes/"+routeID:
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode route update body: %v", err)
			}
			script := body["script"]
			if script == proxyName {
				routeToProxy = true
			}
			if script == workerName {
				routeRestored = true
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"success":true}`))
			return
		case r.Method == http.MethodDelete && r.URL.Path == "/client/v4/accounts/account-123/workers/scripts/"+proxyName:
			proxyDeleted = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"success":true}`))
			return
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer ts.Close()

	oldAPI := cfAPI
	cfAPI = ts.URL + "/client/v4"
	defer func() { cfAPI = oldAPI }()

	oldClient := apiutil.DefaultClient
	apiutil.DefaultClient = ts.Client()
	defer func() { apiutil.DefaultClient = oldClient }()

	t.Setenv("CLOUDFLARE_API_TOKEN", "token")
	t.Setenv("CLOUDFLARE_ACCOUNT_ID", "account-123")
	t.Setenv("CLOUDFLARE_ZONE_ID", zoneID)

	cfg := &config.Config{
		Service: "demo",
		Provider: config.ProviderConfig{
			Name: "cloudflare-workers",
		},
	}

	state, err := RedirectToTunnel(context.Background(), cfg, "dev", "https://abc.ngrok.io")
	if err != nil {
		t.Fatalf("redirect failed: %v", err)
	}
	if state == nil {
		t.Fatal("expected state")
	}
	if !state.Applied {
		t.Fatal("expected applied state")
	}
	if state.EffectiveMode != "route-rewrite" {
		t.Fatalf("expected route-rewrite mode, got %q", state.EffectiveMode)
	}
	if !strings.Contains(state.StatusMessage, "full route rewrite") {
		t.Fatalf("expected route rewrite message, got %q", state.StatusMessage)
	}
	if !proxyUploaded || !routeToProxy {
		t.Fatal("expected proxy upload and route rewrite calls")
	}

	if err := state.Restore(context.Background(), ""); err != nil {
		t.Fatalf("restore failed: %v", err)
	}
	if !routeRestored || !proxyDeleted {
		t.Fatal("expected route restore and proxy delete calls")
	}
}

func TestRedirectToTunnel_CreatesFallbackRouteWhenNoMatchingRoutes(t *testing.T) {
	var (
		proxyUploaded   bool
		fallbackCreated bool
		fallbackDeleted bool
		proxyDeleted    bool
	)

	workerName := "demo-dev"
	proxyName := workerName + "-devstream-proxy"
	zoneID := "zone-123"
	createdRouteID := "created-route-1"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/client/v4/zones/"+zoneID+"/workers/routes":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success":true,"result":[]}`))
			return
		case r.Method == http.MethodPut && r.URL.Path == "/client/v4/accounts/account-123/workers/scripts/"+proxyName:
			proxyUploaded = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"success":true}`))
			return
		case r.Method == http.MethodPost && r.URL.Path == "/client/v4/zones/"+zoneID+"/workers/routes":
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode create route body: %v", err)
			}
			if body["pattern"] != "dev.example.com/*" || body["script"] != proxyName {
				t.Fatalf("unexpected create route payload: %#v", body)
			}
			fallbackCreated = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success":true,"result":{"id":"` + createdRouteID + `","pattern":"dev.example.com/*","script":"` + proxyName + `"}}`))
			return
		case r.Method == http.MethodDelete && r.URL.Path == "/client/v4/zones/"+zoneID+"/workers/routes/"+createdRouteID:
			fallbackDeleted = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"success":true}`))
			return
		case r.Method == http.MethodDelete && r.URL.Path == "/client/v4/accounts/account-123/workers/scripts/"+proxyName:
			proxyDeleted = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"success":true}`))
			return
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer ts.Close()

	oldAPI := cfAPI
	cfAPI = ts.URL + "/client/v4"
	defer func() { cfAPI = oldAPI }()

	oldClient := apiutil.DefaultClient
	apiutil.DefaultClient = ts.Client()
	defer func() { apiutil.DefaultClient = oldClient }()

	t.Setenv("CLOUDFLARE_API_TOKEN", "token")
	t.Setenv("CLOUDFLARE_ACCOUNT_ID", "account-123")
	t.Setenv("CLOUDFLARE_ZONE_ID", zoneID)
	t.Setenv("CLOUDFLARE_DEV_ROUTE_PATTERN", "dev.example.com/*")

	cfg := &config.Config{
		Service: "demo",
		Provider: config.ProviderConfig{
			Name: "cloudflare-workers",
		},
	}

	state, err := RedirectToTunnel(context.Background(), cfg, "dev", "https://abc.ngrok.io")
	if err != nil {
		t.Fatalf("redirect failed: %v", err)
	}
	if state == nil || !state.Applied {
		t.Fatal("expected applied state")
	}
	if !proxyUploaded || !fallbackCreated {
		t.Fatal("expected proxy upload and fallback route creation")
	}
	if len(state.CreatedRoutes) != 1 || state.CreatedRoutes[0].ID != createdRouteID {
		t.Fatalf("expected one created route, got %#v", state.CreatedRoutes)
	}

	if err := state.Restore(context.Background(), ""); err != nil {
		t.Fatalf("restore failed: %v", err)
	}
	if !fallbackDeleted || !proxyDeleted {
		t.Fatal("expected created route and proxy worker cleanup")
	}
}

func TestRedirectToTunnel_NoRouteAndNoPatternFallsBack(t *testing.T) {
	workerName := "demo-dev"
	zoneID := "zone-123"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/client/v4/zones/"+zoneID+"/workers/routes":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success":true,"result":[]}`))
			return
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/workers/scripts/"+workerName+"-devstream-proxy"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"success":true}`))
			return
		case r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/workers/scripts/"+workerName+"-devstream-proxy"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"success":true}`))
			return
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer ts.Close()

	oldAPI := cfAPI
	cfAPI = ts.URL + "/client/v4"
	defer func() { cfAPI = oldAPI }()

	oldClient := apiutil.DefaultClient
	apiutil.DefaultClient = ts.Client()
	defer func() { apiutil.DefaultClient = oldClient }()

	t.Setenv("CLOUDFLARE_API_TOKEN", "token")
	t.Setenv("CLOUDFLARE_ACCOUNT_ID", "account-123")
	t.Setenv("CLOUDFLARE_ZONE_ID", zoneID)
	t.Setenv("CLOUDFLARE_DEV_ROUTE_PATTERN", "")

	cfg := &config.Config{
		Service: "demo",
		Provider: config.ProviderConfig{
			Name: "cloudflare-workers",
		},
	}

	state, err := RedirectToTunnel(context.Background(), cfg, "dev", "https://abc.ngrok.io")
	if err != nil {
		t.Fatalf("redirect failed: %v", err)
	}
	if state == nil {
		t.Fatal("expected state")
	}
	if state.Applied {
		t.Fatal("expected fallback state without applied rewrite")
	}
	if state.EffectiveMode != "lifecycle-only" {
		t.Fatalf("expected lifecycle fallback, got %q", state.EffectiveMode)
	}
	if !strings.Contains(state.StatusMessage, "no matching Cloudflare routes were found and no fallback pattern is configured") {
		t.Fatalf("unexpected status: %q", state.StatusMessage)
	}
}

package cloudflare

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	sdkrouter "github.com/runfabric/runfabric/plugin-sdk/go/router"
)

func TestCloudflareSyncer_DNSNoOpWhenAlreadyAligned(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/zones/zone-1/dns_records" {
			writeCloudflareSuccess(t, w, []cloudflareDNSRecord{
				{
					ID:      "rec-1",
					Type:    "CNAME",
					Name:    "svc.example.com",
					Content: "aws.example.com",
					TTL:     300,
					Comment: managedByTag,
				},
			})
			return
		}
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
	}))
	defer srv.Close()

	syncer := newTestCloudflareSyncer(t, srv, false)
	result, err := syncer.sync(context.Background(), testRoutingConfig())
	if err != nil {
		t.Fatalf("sync returned error: %v", err)
	}
	if len(result.Actions) != 1 {
		t.Fatalf("expected one action, got %#v", result.Actions)
	}
	if result.Actions[0].Action != "no-op" || result.Actions[0].Resource != "dns_record" {
		t.Fatalf("unexpected action: %#v", result.Actions[0])
	}
}

func TestCloudflareSyncer_DNSDryRunUpdateWhenTargetDiffers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/zones/zone-1/dns_records" {
			writeCloudflareSuccess(t, w, []cloudflareDNSRecord{
				{
					ID:      "rec-1",
					Type:    "CNAME",
					Name:    "svc.example.com",
					Content: "old.example.com",
					TTL:     120,
					Comment: managedByTag,
				},
			})
			return
		}
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
	}))
	defer srv.Close()

	syncer := newTestCloudflareSyncer(t, srv, true)
	result, err := syncer.sync(context.Background(), testRoutingConfig())
	if err != nil {
		t.Fatalf("sync returned error: %v", err)
	}
	if len(result.Actions) != 1 {
		t.Fatalf("expected one action, got %#v", result.Actions)
	}
	if result.Actions[0].Action != "update" || result.Actions[0].Resource != "dns_record" {
		t.Fatalf("unexpected action: %#v", result.Actions[0])
	}
}

func TestCloudflareSyncer_DNSFailureBubblesUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/zones/zone-1/dns_records" {
			writeCloudflareError(t, w, 1001, "bad token")
			return
		}
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
	}))
	defer srv.Close()

	syncer := newTestCloudflareSyncer(t, srv, false)
	_, err := syncer.sync(context.Background(), testRoutingConfig())
	if err == nil {
		t.Fatal("expected sync error")
	}
	if !strings.Contains(err.Error(), "sync dns record") {
		t.Fatalf("expected dns sync context in error, got: %v", err)
	}
}

func TestCloudflareClient_RetriesTransientHTTPFailures(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/zones/zone-1/dns_records" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		current := atomic.AddInt32(&attempts, 1)
		if current < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"success":false}`))
			return
		}
		writeCloudflareSuccess(t, w, []cloudflareDNSRecord{})
	}))
	defer srv.Close()

	client := &cloudflareClient{
		token:      strings.Repeat("a", 24),
		zoneID:     "zone-1",
		baseURL:    srv.URL,
		httpClient: srv.Client(),
	}
	records, err := client.listDNSRecords(context.Background(), "svc.example.com", "CNAME")
	if err != nil {
		t.Fatalf("listDNSRecords returned error: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("expected empty records, got %#v", records)
	}
	if got := atomic.LoadInt32(&attempts); got != 3 {
		t.Fatalf("expected 3 attempts, got %d", got)
	}
}

func TestOriginsEqual_DetectsWeightDiff(t *testing.T) {
	a := []cloudflareLBOrigin{{Name: "aws", Address: "https://aws.example.com", Weight: 100}}
	b := []cloudflareLBOrigin{{Name: "aws", Address: "https://aws.example.com", Weight: 20}}
	if originsEqual(a, b) {
		t.Fatal("expected different origin weights to be treated as changed")
	}
}

func newTestCloudflareSyncer(t *testing.T, srv *httptest.Server, dryRun bool) *cloudflareSyncer {
	t.Helper()
	syncer, err := newCloudflareSyncer(cloudflareConfig{
		APIToken: strings.Repeat("a", 24),
		ZoneID:   "zone-1",
	}, dryRun, nil)
	if err != nil {
		t.Fatalf("newCloudflareSyncer returned error: %v", err)
	}
	syncer.client.baseURL = srv.URL
	syncer.client.httpClient = srv.Client()
	return syncer
}

func testRoutingConfig() *sdkrouter.RoutingConfig {
	return &sdkrouter.RoutingConfig{
		Contract: "runfabric.fabric.routing.v1",
		Service:  "svc",
		Stage:    "dev",
		Hostname: "svc.example.com",
		Strategy: "round-robin",
		TTL:      300,
		Endpoints: []sdkrouter.RoutingEndpoint{
			{Name: "aws", URL: "https://aws.example.com", Weight: 100},
		},
	}
}

func writeCloudflareSuccess[T any](t *testing.T, w http.ResponseWriter, result T) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(cloudflareResponse[T]{
		Success: true,
		Result:  result,
		Errors:  []cloudflareErr{},
	}); err != nil {
		t.Fatalf("encode success response: %v", err)
	}
}

func writeCloudflareError(t *testing.T, w http.ResponseWriter, code int, message string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(cloudflareResponse[map[string]any]{
		Success: false,
		Errors: []cloudflareErr{
			{Code: code, Message: message},
		},
	}); err != nil {
		t.Fatalf("encode error response: %v", err)
	}
}

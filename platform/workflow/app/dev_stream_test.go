package app

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/runfabric/runfabric/platform/deploy/apiutil"
	"github.com/runfabric/runfabric/platform/observability/diagnostics"
)

func TestPrepareDevStreamTunnel_BuiltInProvidersReturnRestore(t *testing.T) {
	providers := []struct {
		name   string
		region string
	}{
		{name: "gcp-functions", region: "us-central1"},
		{name: "cloudflare-workers"},
		{name: "azure-functions", region: "westus2"},
		{name: "digitalocean-functions"},
		{name: "fly-machines"},
		{name: "kubernetes"},
		{name: "netlify"},
		{name: "vercel"},
		{name: "alibaba-fc", region: "cn-hangzhou"},
		{name: "ibm-openwhisk"},
	}

	for _, tc := range providers {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			cfg := "service: test-dev-stream\n" +
				"provider:\n" +
				"  name: " + tc.name + "\n" +
				"  runtime: nodejs\n"
			if tc.region != "" {
				cfg += "  region: " + tc.region + "\n"
			}
			cfg += "functions:\n" +
				"  - name: api\n" +
				"    entry: index.handler\n"
			cfgPath := filepath.Join(dir, "runfabric.yml")
			if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
				t.Fatal(err)
			}
			restore, err := PrepareDevStreamTunnel(cfgPath, "dev", "https://abc.ngrok.io")
			if err != nil {
				t.Fatalf("expected no error for %s provider, got %v", tc.name, err)
			}
			if restore == nil {
				t.Fatalf("expected non-nil restore for %s provider hook", tc.name)
			}
			restore()
		})
	}
}

func TestPrepareDevStreamTunnel_UnknownProviderReturnsNil(t *testing.T) {
	dir := t.TempDir()
	cfg := "service: test-dev-stream\n" +
		"provider:\n" +
		"  name: unknown-provider\n" +
		"  runtime: nodejs\n" +
		"functions:\n" +
		"  - name: api\n" +
		"    entry: index.handler\n"
	cfgPath := filepath.Join(dir, "runfabric.yml")
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	restore, err := PrepareDevStreamTunnel(cfgPath, "dev", "https://abc.ngrok.io")
	if err != nil {
		t.Fatalf("expected no error for unknown provider, got %v", err)
	}
	if restore != nil {
		t.Fatal("expected nil restore for unknown provider")
	}
}

func TestPrepareDevStreamTunnelWithReport_GCPMissingPrereqsFallsBack(t *testing.T) {
	dir := t.TempDir()
	cfg := "service: test-dev-stream\n" +
		"provider:\n" +
		"  name: gcp-functions\n" +
		"  runtime: nodejs\n" +
		"  region: us-central1\n" +
		"functions:\n" +
		"  - name: api\n" +
		"    entry: index.handler\n"
	cfgPath := filepath.Join(dir, "runfabric.yml")
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GCP_ACCESS_TOKEN", "")
	t.Setenv("GCP_PROJECT", "")
	t.Setenv("GCP_PROJECT_ID", "")
	restore, report, err := PrepareDevStreamTunnelWithReport(cfgPath, "dev", "https://localhost.example.test")
	if err != nil {
		t.Fatalf("expected no error for gcp fallback, got %v", err)
	}
	if restore == nil {
		t.Fatal("expected restore function for gcp provider hook")
	}
	if report == nil {
		t.Fatal("expected dev-stream report")
	}
	if report.CapabilityMode != "conditional-mutation" {
		t.Fatalf("expected conditional-mutation capability, got %q", report.CapabilityMode)
	}
	if report.EffectiveMode != "lifecycle-only" {
		t.Fatalf("expected lifecycle-only fallback, got %q", report.EffectiveMode)
	}
	if len(report.MissingPrereqs) == 0 {
		t.Fatal("expected missing prerequisite details")
	}
	if !strings.Contains(report.Message, "falling back to lifecycle-only") {
		t.Fatalf("expected fallback message, got %q", report.Message)
	}
	restore()
}

func TestDevStreamDoctorAddsCapabilityChecks(t *testing.T) {
	dir := t.TempDir()
	cfg := "service: test-dev-stream\n" +
		"provider:\n" +
		"  name: cloudflare-workers\n" +
		"  runtime: nodejs\n" +
		"functions:\n" +
		"  - name: api\n" +
		"    entry: index.handler\n"
	cfgPath := filepath.Join(dir, "runfabric.yml")
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	t.Setenv("CLOUDFLARE_ACCOUNT_ID", "")
	result, err := DevStreamDoctor(cfgPath, "dev", "https://localhost:3000")
	if err != nil {
		t.Fatalf("expected no error from dev-stream doctor, got %v", err)
	}
	report, ok := result.(*diagnostics.HealthReport)
	if !ok {
		t.Fatalf("expected health report, got %T", result)
	}
	checks := map[string]string{}
	for _, check := range report.Checks {
		checks[check.Name] = check.Message
	}
	if _, ok := checks["dev-stream-capability"]; !ok {
		t.Fatal("expected dev-stream-capability check")
	}
	if message, ok := checks["dev-stream-provider-mutation"]; !ok || !strings.Contains(message, "lifecycle-only fallback") {
		t.Fatalf("expected mutation fallback check, got %q", message)
	}
	if _, ok := checks["dev-stream-tunnel-url"]; !ok {
		t.Fatal("expected dev-stream-tunnel-url check")
	}
}

func TestPrepareDevStreamTunnelWithReport_AzureGatewayHooksRouteRewrite(t *testing.T) {
	dir := t.TempDir()
	cfg := "service: test-dev-stream\n" +
		"provider:\n" +
		"  name: azure-functions\n" +
		"  runtime: nodejs\n" +
		"  region: westus2\n" +
		"functions:\n" +
		"  - name: api\n" +
		"    entry: index.handler\n"
	cfgPath := filepath.Join(dir, "runfabric.yml")
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	setCalled := false
	restoreCalled := false
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/set":
			setCalled = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		case "/restore":
			restoreCalled = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer gateway.Close()

	oldClient := apiutil.DefaultClient
	apiutil.DefaultClient = gateway.Client()
	defer func() { apiutil.DefaultClient = oldClient }()

	t.Setenv("RUNFABRIC_DEV_STREAM_AZURE_FUNCTIONS_SET_URL", gateway.URL+"/set")
	t.Setenv("RUNFABRIC_DEV_STREAM_AZURE_FUNCTIONS_RESTORE_URL", gateway.URL+"/restore")
	t.Setenv("RUNFABRIC_DEV_STREAM_AZURE_FUNCTIONS_TOKEN", "")

	restore, report, err := PrepareDevStreamTunnelWithReport(cfgPath, "dev", "https://localhost.example.test")
	if err != nil {
		t.Fatalf("expected no error for azure gateway hooks, got %v", err)
	}
	if restore == nil {
		t.Fatal("expected restore function")
	}
	if report == nil {
		t.Fatal("expected report")
	}
	if report.EffectiveMode != "route-rewrite" {
		t.Fatalf("expected route-rewrite effective mode, got %q", report.EffectiveMode)
	}
	if !strings.Contains(report.Message, "route rewrite applied") {
		t.Fatalf("unexpected report message: %q", report.Message)
	}
	if !setCalled {
		t.Fatal("expected gateway set hook call")
	}

	restore()
	if !restoreCalled {
		t.Fatal("expected gateway restore hook call")
	}
}

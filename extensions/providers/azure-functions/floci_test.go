//go:build floci

// Package azure floci integration test. Exercises the provider's contract methods
// DIRECTLY (no CLI/daemon binaries) against a floci-az emulator, so it is
// self-contained and travels with the provider when this package is extracted to
// its own repository.
//
// Scope note: floci-az faithfully emulates the Azure Resource Manager (ARM)
// control plane for function apps (resource-group + Microsoft.Web/sites create,
// poll, delete), so this test covers the ARM lifecycle the provider drives:
// Deploy (create app) -> the app is live in ARM -> Remove -> it is gone.
//
// It does NOT assert a real invocation, because floci-az's function runtime is
// impractical to drive here: it accepts only a non-standard admin deploy API
// (not the Kudu zip deploy the provider uses in production, which it answers 501)
// and it launches mcr.microsoft.com/azure-functions/* runtime images that have
// no arm64 manifest. The real code-push + invoke path is validated instead by
// TestDeployPushesCodeAndInvokeReturnsPayload against a faithful ARM+Kudu+function
// httptest double (always-on, no emulator required).
//
// Run with a floci-az reachable on AZURE_ENDPOINT_URL (default http://localhost:4577):
//
//	go test -tags floci ./extensions/providers/azure-functions/... -run Floci -v
//
// Or let the test start (and stop) its own container:
//
//	RUNFABRIC_FLOCI_DOCKER=1 go test -tags floci ./extensions/providers/azure-functions/... -run Floci -v
package azure

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

const flociAZImage = "floci/floci-az:latest"

const flociAZSubID = "00000000-0000-0000-0000-000000000000"

func azConfig(service string) sdkprovider.Config {
	return sdkprovider.Config{
		"service": service,
		"provider": map[string]any{
			"name":    "azure-functions",
			"runtime": "node",
			"region":  "westus2",
		},
		"functions": map[string]any{
			"hello": map[string]any{"handler": "index.handler"},
		},
	}
}

func TestFlociAZLifecycle(t *testing.T) {
	endpoint := ensureFlociAZ(t)
	t.Setenv("AZURE_ENDPOINT_URL", endpoint)
	t.Setenv("AZURE_ACCESS_TOKEN", "test")
	t.Setenv("AZURE_SUBSCRIPTION_ID", flociAZSubID)
	t.Setenv("AZURE_RESOURCE_GROUP", "")

	ctx := context.Background()
	stage := "dev"
	service := "e2e-az"
	cfg := azConfig(service)
	appName := fmt.Sprintf("%s-%s", service, stage)

	root := t.TempDir()
	if err := os.WriteFile(root+"/index.js", []byte("module.exports = async (c, req) => { c.res = { body: req.body }; };\n"), 0o644); err != nil {
		t.Fatalf("write handler: %v", err)
	}

	t.Cleanup(func() {
		_, _ = Remover{}.Remove(ctx, cfg, stage, root, nil)
	})

	// --- Deploy: creates the resource group + function app via ARM ---
	dep, err := Runner{}.Deploy(ctx, cfg, stage, root)
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	if got := dep.Outputs["app_name"]; got != appName {
		t.Errorf("app_name = %q, want %q", got, appName)
	}
	fn, ok := dep.Functions["hello"]
	if !ok || fn.ResourceName != appName+"/hello" {
		t.Errorf("Deploy did not return the 'hello' function mapping: %+v", dep.Functions)
	}
	// floci-az does not implement Kudu zip deploy, so the best-effort code-push is
	// expected to be recorded as skipped rather than failing the deploy.
	if cd := dep.Outputs["code_deploy"]; cd == "" {
		t.Errorf("code_deploy status not recorded")
	}

	// --- The app is live in the ARM control plane ---
	if code := azSiteStatus(t, endpoint, flociAZSubID, appName, appName); code != http.StatusOK {
		t.Errorf("site GET after Deploy = %d, want 200 (control-plane presence)", code)
	}

	// --- Remove: the ARM resource is gone afterward ---
	rem, err := Remover{}.Remove(ctx, cfg, stage, root, sdkprovider.ReceiptView{Outputs: dep.Outputs})
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if !rem.Removed {
		t.Errorf("Remove did not report Removed")
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if azSiteStatus(t, endpoint, flociAZSubID, appName, appName) == http.StatusNotFound {
			return
		}
		time.Sleep(1 * time.Second)
	}
	t.Errorf("function app still present after Remove (GET != 404)")
}

// azSiteStatus returns the HTTP status of an ARM Microsoft.Web/sites GET, used to
// assert control-plane presence/absence.
func azSiteStatus(t *testing.T, endpoint, subID, rg, appName string) int {
	t.Helper()
	u := fmt.Sprintf("%s/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Web/sites/%s?api-version=2022-03-01",
		strings.TrimRight(endpoint, "/"), subID, rg, appName)
	req, _ := http.NewRequest(http.MethodGet, u, nil)
	req.Header.Set("Authorization", "Bearer test")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("site GET: %v", err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

// ensureFlociAZ returns a reachable floci-az endpoint: an already-running one on
// AZURE_ENDPOINT_URL, or a container it starts (RUNFABRIC_FLOCI_DOCKER=1). It
// skips the test when neither is available.
func ensureFlociAZ(t *testing.T) string {
	t.Helper()
	endpoint := strings.TrimSpace(os.Getenv("AZURE_ENDPOINT_URL"))
	if endpoint == "" {
		endpoint = "http://localhost:4577"
	}
	if azTCPReachable(endpoint) {
		return endpoint
	}
	if os.Getenv("RUNFABRIC_FLOCI_DOCKER") != "1" {
		t.Skipf("no floci-az reachable at %s; set RUNFABRIC_FLOCI_DOCKER=1 to start one, or point AZURE_ENDPOINT_URL at a running emulator", endpoint)
	}
	return startFlociAZContainer(t)
}

func startFlociAZContainer(t *testing.T) string {
	t.Helper()
	if _, err := exec.Command("docker", "version").CombinedOutput(); err != nil {
		t.Skip("docker not available")
	}
	image := flociAZImage
	if v := os.Getenv("FLOCI_AZ_IMAGE"); v != "" {
		image = v
	}
	// The ARM control plane needs no runtime containers, so no docker.sock mount.
	out, err := exec.Command("docker", "run", "-d", "--rm", "-p", "0:4577", image).CombinedOutput()
	if err != nil {
		t.Skipf("could not start floci-az: %v\n%s", err, out)
	}
	id := strings.TrimSpace(string(out))
	t.Cleanup(func() { _, _ = exec.Command("docker", "stop", id).CombinedOutput() })

	portOut, err := exec.Command("docker", "port", id, "4577/tcp").CombinedOutput()
	if err != nil {
		t.Fatalf("docker port: %v\n%s", err, portOut)
	}
	line := strings.TrimSpace(strings.SplitN(string(portOut), "\n", 2)[0])
	port := line[strings.LastIndex(line, ":")+1:]
	endpoint := "http://127.0.0.1:" + strings.TrimSpace(port)

	deadline := time.Now().Add(120 * time.Second)
	client := &http.Client{Timeout: 3 * time.Second}
	for time.Now().Before(deadline) {
		if resp, err := client.Get(endpoint + "/health"); err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return endpoint
			}
		}
		time.Sleep(1 * time.Second)
	}
	t.Fatalf("floci-az did not become ready at %s", endpoint)
	return ""
}

func azTCPReachable(endpoint string) bool {
	u, err := url.Parse(endpoint)
	if err != nil {
		return false
	}
	host := u.Host
	if u.Port() == "" {
		host += ":80"
	}
	conn, err := net.DialTimeout("tcp", host, 1500*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

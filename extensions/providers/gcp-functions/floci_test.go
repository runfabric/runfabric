//go:build floci

// Package gcp floci integration test. Exercises the provider's contract methods
// DIRECTLY (no CLI/daemon binaries) against a floci-gcp emulator, so it is
// self-contained and travels with the provider when this package is extracted
// to its own repository.
//
// Scope note: floci-gcp emulates the Cloud Functions *control plane* (create,
// operations/LRO, get, delete) and GCS, but NOT the function runtime — there is
// no real invocation, logging, or metrics backend. So this test covers the
// honest lifecycle the emulator supports: GCS source upload -> Deploy -> the
// function is live in the control plane -> Remove -> it is gone. (The AWS
// Floci test additionally invokes, tails logs, and rolls back because the AWS
// emulator runs a real Lambda runtime; floci-gcp does not.)
//
// Run with a floci-gcp reachable on GCP_ENDPOINT_URL (default http://localhost:4588):
//
//	go test -tags floci ./extensions/providers/gcp-functions/... -run Floci -v
//
// Or let the test start (and stop) its own container:
//
//	RUNFABRIC_FLOCI_DOCKER=1 go test -tags floci ./extensions/providers/gcp-functions/... -run Floci -v
package gcp

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

const flociGCPImage = "floci/floci-gcp:latest"

// gcpConfig builds a resolved provider config with one function. GCP Cloud
// Functions has no linear-chain orchestration analogous to AWS Step Functions
// exercised here (floci-gcp does not emulate Workflows), so the config stays
// minimal.
func gcpConfig(service string) sdkprovider.Config {
	return sdkprovider.Config{
		"service": service,
		"provider": map[string]any{
			"name":    "gcp-functions",
			"runtime": "nodejs20",
			"region":  "us-central1",
		},
		"functions": map[string]any{
			"hello": map[string]any{"handler": "index.handler"},
		},
	}
}

func TestFlociGCPLifecycle(t *testing.T) {
	endpoint := ensureFlociGCP(t)
	t.Setenv("GCP_ENDPOINT_URL", endpoint)
	t.Setenv("GCP_ACCESS_TOKEN", "test")
	t.Setenv("GCP_PROJECT", "test-project")
	t.Setenv("GCP_UPLOAD_BUCKET", "rf-src")

	ctx := context.Background()
	stage := "dev"
	service := "e2e-gcp"
	cfg := gcpConfig(service)
	funcName := fmt.Sprintf("%s-%s-hello", service, stage)

	root := t.TempDir()
	writeGCPHandler(t, root, "exports.handler = (req, res) => res.status(200).send('ok');\n")

	// floci-gcp does not auto-create buckets; the provider uploads the zip but
	// does not create the bucket, so seed it first (mirrors real GCP where the
	// GCS source bucket must pre-exist).
	createGCSBucket(t, endpoint, "test-project", "rf-src")

	// Ensure teardown regardless of failures.
	t.Cleanup(func() {
		_, _ = Remover{}.Remove(ctx, cfg, stage, root, nil)
	})

	// --- Deploy: uploads source to GCS, creates the function, waits on the LRO ---
	dep, err := Runner{}.Deploy(ctx, cfg, stage, root)
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	fn, ok := dep.Functions["hello"]
	if !ok {
		t.Fatalf("Deploy did not return the 'hello' function: %+v", dep.Functions)
	}
	if fn.ResourceName != funcName {
		t.Errorf("ResourceName = %q, want %q", fn.ResourceName, funcName)
	}
	if !strings.Contains(fn.ResourceIdentifier, "functions/"+funcName) &&
		!strings.Contains(fn.ResourceIdentifier, funcName) {
		t.Errorf("ResourceIdentifier missing the function name: %q", fn.ResourceIdentifier)
	}

	// --- The function is live in the control plane (state ACTIVE) ---
	if code := gcpFunctionStatus(t, endpoint, "test-project", "us-central1", funcName); code != http.StatusOK {
		t.Errorf("function GET after Deploy = %d, want 200 (control-plane presence)", code)
	}

	// --- Remove: the control-plane resource is gone afterward ---
	rem, err := Remover{}.Remove(ctx, cfg, stage, root, sdkprovider.ReceiptView{Outputs: dep.Outputs})
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if !rem.Removed {
		t.Errorf("Remove did not report Removed")
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if gcpFunctionStatus(t, endpoint, "test-project", "us-central1", funcName) == http.StatusNotFound {
			return
		}
		time.Sleep(1 * time.Second)
	}
	t.Errorf("function still present after Remove (GET != 404)")
}

func writeGCPHandler(t *testing.T, root, body string) {
	t.Helper()
	if err := os.WriteFile(root+"/index.js", []byte(body), 0o644); err != nil {
		t.Fatalf("write handler: %v", err)
	}
}

// createGCSBucket creates a GCS bucket via the JSON API so the provider's source
// upload has a destination.
func createGCSBucket(t *testing.T, endpoint, project, name string) {
	t.Helper()
	u := strings.TrimRight(endpoint, "/") + "/storage/v1/b?project=" + url.QueryEscape(project)
	body := fmt.Sprintf(`{"name":%q}`, name)
	req, err := http.NewRequest(http.MethodPost, u, strings.NewReader(body))
	if err != nil {
		t.Fatalf("build create-bucket request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("create bucket: %v", err)
	}
	defer resp.Body.Close()
	// 200 (created) or 409 (already exists) are both fine.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusConflict {
		t.Fatalf("create bucket %s: unexpected status %s", name, resp.Status)
	}
}

// gcpFunctionStatus returns the HTTP status of a Cloud Functions v2 GET, used to
// assert control-plane presence/absence.
func gcpFunctionStatus(t *testing.T, endpoint, project, region, funcName string) int {
	t.Helper()
	u := fmt.Sprintf("%s/v2/projects/%s/locations/%s/functions/%s",
		strings.TrimRight(endpoint, "/"), project, region, funcName)
	req, _ := http.NewRequest(http.MethodGet, u, nil)
	req.Header.Set("Authorization", "Bearer test")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("function GET: %v", err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

// ensureFlociGCP returns a reachable floci-gcp endpoint: an already-running one
// on GCP_ENDPOINT_URL, or a container it starts (RUNFABRIC_FLOCI_DOCKER=1). It
// skips the test when neither is available.
func ensureFlociGCP(t *testing.T) string {
	t.Helper()
	endpoint := strings.TrimSpace(os.Getenv("GCP_ENDPOINT_URL"))
	if endpoint == "" {
		endpoint = "http://localhost:4588"
	}
	if gcpTCPReachable(endpoint) {
		return endpoint
	}
	if os.Getenv("RUNFABRIC_FLOCI_DOCKER") != "1" {
		t.Skipf("no floci-gcp reachable at %s; set RUNFABRIC_FLOCI_DOCKER=1 to start one, or point GCP_ENDPOINT_URL at a running emulator", endpoint)
	}
	return startFlociGCPContainer(t)
}

func startFlociGCPContainer(t *testing.T) string {
	t.Helper()
	if _, err := exec.Command("docker", "version").CombinedOutput(); err != nil {
		t.Skip("docker not available")
	}
	image := flociGCPImage
	if v := os.Getenv("FLOCI_GCP_IMAGE"); v != "" {
		image = v
	}
	// floci-gcp is control-plane only — no runtime containers — so unlike the
	// AWS emulator it needs no docker.sock mount.
	out, err := exec.Command("docker", "run", "-d", "--rm", "-p", "0:4588", image).CombinedOutput()
	if err != nil {
		t.Skipf("could not start floci-gcp: %v\n%s", err, out)
	}
	id := strings.TrimSpace(string(out))
	t.Cleanup(func() { _, _ = exec.Command("docker", "stop", id).CombinedOutput() })

	portOut, err := exec.Command("docker", "port", id, "4588/tcp").CombinedOutput()
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
	t.Fatalf("floci-gcp did not become ready at %s", endpoint)
	return ""
}

func gcpTCPReachable(endpoint string) bool {
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

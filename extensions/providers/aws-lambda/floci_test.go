//go:build floci

// Package aws floci integration test. Exercises the provider's contract methods
// DIRECTLY (no CLI/daemon binaries) against a Floci AWS emulator, so it is
// self-contained and travels with the provider when this package is extracted
// to its own repository.
//
// Run with a Floci reachable on AWS_ENDPOINT_URL (default http://localhost:4566):
//
//	go test -tags floci ./extensions/providers/aws-lambda/... -run Floci -v
//
// Or let the test start (and stop) its own container:
//
//	RUNFABRIC_FLOCI_DOCKER=1 go test -tags floci ./extensions/providers/aws-lambda/... -run Floci -v
//
// It skips when neither a reachable endpoint nor Docker (with opt-in) is present.
package aws

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	lambdav2 "github.com/aws/aws-sdk-go-v2/service/lambda"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

const flociImage = "floci/floci:1.5.30"

// lambdaConfig builds a resolved provider config with one http function and an
// all-code workflow that chains it (for the orchestration checks).
func lambdaConfig(service string) sdkprovider.Config {
	return sdkprovider.Config{
		"service": service,
		"provider": map[string]any{
			"name":    "aws-lambda",
			"runtime": "nodejs20.x",
			"region":  "us-east-1",
		},
		"functions": map[string]any{
			"hello": map[string]any{
				"handler": "index.handler",
				"triggers": []any{
					map[string]any{"type": "http", "method": "GET", "path": "/hello"},
				},
			},
		},
		"workflows": []any{
			map[string]any{
				"name": "chain",
				"steps": []any{
					map[string]any{"id": "run", "kind": "code", "input": map[string]any{"function": "hello"}},
				},
			},
		},
	}
}

func TestFlociProviderLifecycle(t *testing.T) {
	endpoint := ensureFloci(t)
	t.Setenv("AWS_ENDPOINT_URL", endpoint)
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	t.Setenv("AWS_REGION", "us-east-1")

	ctx := context.Background()
	p := &Provider{}
	stage := "dev"
	cfg := lambdaConfig("e2e-lambda")

	root := t.TempDir()
	writeHandler(t, root, "exports.handler = async (e) => { console.log('handler invoked'); return ({ statusCode: 200, body: JSON.stringify({ ok: true, echo: e }) }); };\n")

	// Ensure teardown regardless of failures.
	t.Cleanup(func() {
		_, _ = p.RemoveOrchestrations(ctx, sdkprovider.OrchestrationRemoveRequest{Config: cfg, Stage: stage})
		_, _ = p.Remove(ctx, sdkprovider.RemoveRequest{Config: cfg, Stage: stage})
	})

	// --- Deploy ---
	dep, err := p.Deploy(ctx, sdkprovider.DeployRequest{Config: cfg, Stage: stage, Root: root})
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	fn, ok := dep.Functions["hello"]
	if !ok || !strings.Contains(fn.ResourceIdentifier, "arn:aws:lambda") {
		t.Fatalf("Deploy did not return a Lambda ARN: %+v", dep.Functions)
	}

	// --- Invoke (retry through the runtime-image cold start) ---
	var out string
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		res, invErr := p.Invoke(ctx, sdkprovider.InvokeRequest{
			Config: cfg, Stage: stage, Function: "hello", Payload: []byte(`{"n":1}`),
		})
		if invErr == nil {
			out = res.Output
			if strings.Contains(out, `"statusCode":200`) && strings.Contains(out, "ok") {
				break
			}
		}
		time.Sleep(2 * time.Second)
	}
	if !strings.Contains(out, `"statusCode":200`) {
		t.Fatalf("Invoke did not return the handler payload; got %q", out)
	}

	// --- Logs (real CloudWatch lines; poll for propagation) ---
	var logs *sdkprovider.LogsResult
	logsDeadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(logsDeadline) {
		logs, err = p.Logs(ctx, sdkprovider.LogsRequest{Config: cfg, Stage: stage, Function: "hello"})
		if err != nil {
			t.Fatalf("Logs: %v", err)
		}
		if len(logs.Lines) > 0 {
			break
		}
		time.Sleep(2 * time.Second)
	}
	if len(logs.Lines) == 0 {
		t.Errorf("Logs returned no lines after an invocation")
	}
	for _, l := range logs.Lines {
		if strings.Contains(l, "available in CloudWatch") {
			t.Errorf("Logs returned the stub message: %q", l)
		}
	}

	// --- Metrics (real structure) ---
	metrics, err := p.FetchMetrics(ctx, sdkprovider.MetricsRequest{Config: cfg, Stage: stage})
	if err != nil {
		t.Fatalf("FetchMetrics: %v", err)
	}
	hm, _ := metrics.PerFunction["hello"].(map[string]any)
	if _, has := hm["invocations"]; !has {
		t.Errorf("FetchMetrics missing per-function invocations: %+v", metrics.PerFunction)
	}

	// --- Orchestration: sync creates a state machine; invoke + inspect work ---
	fnByName := map[string]string{"hello": fn.ResourceIdentifier}
	sync, err := p.SyncOrchestrations(ctx, sdkprovider.OrchestrationSyncRequest{
		Config: cfg, Stage: stage, FunctionResourceByName: fnByName,
	})
	if err != nil {
		t.Fatalf("SyncOrchestrations: %v", err)
	}
	if sync.Outputs["chain"] == "" {
		t.Errorf("SyncOrchestrations did not create a state machine for 'chain': %+v", sync)
	}
	oi, err := p.InvokeOrchestration(ctx, sdkprovider.OrchestrationInvokeRequest{
		Config: cfg, Stage: stage, Name: "chain", Payload: []byte("{}"),
	})
	if err != nil {
		t.Fatalf("InvokeOrchestration: %v", err)
	}
	if !strings.Contains(oi.Output, "execution") {
		t.Errorf("InvokeOrchestration returned no execution ARN: %q", oi.Output)
	}
	insp, err := p.InspectOrchestrations(ctx, sdkprovider.OrchestrationInspectRequest{Config: cfg, Stage: stage})
	if err != nil {
		t.Fatalf("InspectOrchestrations: %v", err)
	}
	if machines, _ := insp["stateMachines"].([]any); len(machines) == 0 {
		t.Errorf("InspectOrchestrations found no state machines")
	}

	// --- Rollback: a second deploy publishes v2; recover reverts to v1 ---
	writeHandler(t, root, "exports.handler = async () => ({ statusCode: 200, body: '{\"v\":2}' });\n")
	if _, err := p.Deploy(ctx, sdkprovider.DeployRequest{Config: cfg, Stage: stage, Root: root}); err != nil {
		t.Fatalf("Deploy v2: %v", err)
	}
	rec, err := p.Recover(ctx, sdkprovider.RecoveryRequest{Config: cfg, Stage: stage, Mode: "rollback"})
	if err != nil {
		t.Fatalf("Recover rollback: %v", err)
	}
	if !rec.Recovered || !strings.Contains(rec.Metadata["rollback:hello"], "->") {
		t.Errorf("rollback did not revert to a previous version: %+v", rec.Metadata)
	}

	// --- Remove: function is gone afterward ---
	rem, err := p.Remove(ctx, sdkprovider.RemoveRequest{Config: cfg, Stage: stage})
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if !rem.Removed {
		t.Errorf("Remove did not report Removed")
	}
	clients, err := loadClients(ctx, "us-east-1")
	if err != nil {
		t.Fatalf("loadClients: %v", err)
	}
	if _, err := clients.Lambda.GetFunction(ctx, &lambdav2.GetFunctionInput{
		FunctionName: awssdk.String("e2e-lambda-dev-hello"),
	}); !isLambdaNotFound(err) {
		t.Errorf("function still exists after Remove (GetFunction err = %v)", err)
	}
}

func writeHandler(t *testing.T, root, body string) {
	t.Helper()
	if err := os.WriteFile(root+"/index.js", []byte(body), 0o644); err != nil {
		t.Fatalf("write handler: %v", err)
	}
}

// ensureFloci returns a reachable Floci endpoint: an already-running one on
// AWS_ENDPOINT_URL, or a container it starts (RUNFABRIC_FLOCI_DOCKER=1). It
// skips the test when neither is available.
func ensureFloci(t *testing.T) string {
	t.Helper()
	endpoint := strings.TrimSpace(os.Getenv("AWS_ENDPOINT_URL"))
	if endpoint == "" {
		endpoint = "http://localhost:4566"
	}
	if tcpReachable(endpoint) {
		return endpoint
	}
	if os.Getenv("RUNFABRIC_FLOCI_DOCKER") != "1" {
		t.Skipf("no Floci reachable at %s; set RUNFABRIC_FLOCI_DOCKER=1 to start one, or point AWS_ENDPOINT_URL at a running emulator", endpoint)
	}
	return startFlociContainer(t)
}

func startFlociContainer(t *testing.T) string {
	t.Helper()
	if _, err := exec.Command("docker", "version").CombinedOutput(); err != nil {
		t.Skip("docker not available")
	}
	image := flociImage
	if v := os.Getenv("FLOCI_IMAGE"); v != "" {
		image = v
	}
	out, err := exec.Command("docker", "run", "-d", "--rm",
		"-p", "0:4566",
		"-v", "/var/run/docker.sock:/var/run/docker.sock",
		image).CombinedOutput()
	if err != nil {
		t.Skipf("could not start Floci: %v\n%s", err, out)
	}
	id := strings.TrimSpace(string(out))
	t.Cleanup(func() { _, _ = exec.Command("docker", "stop", id).CombinedOutput() })

	portOut, err := exec.Command("docker", "port", id, "4566/tcp").CombinedOutput()
	if err != nil {
		t.Fatalf("docker port: %v\n%s", err, portOut)
	}
	line := strings.TrimSpace(strings.SplitN(string(portOut), "\n", 2)[0])
	port := line[strings.LastIndex(line, ":")+1:]
	endpoint := "http://127.0.0.1:" + strings.TrimSpace(port)

	deadline := time.Now().Add(120 * time.Second)
	client := &http.Client{Timeout: 3 * time.Second}
	for time.Now().Before(deadline) {
		if resp, err := client.Get(endpoint + "/_localstack/health"); err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return endpoint
			}
		}
		time.Sleep(1 * time.Second)
	}
	t.Fatalf("Floci did not become ready at %s", endpoint)
	return ""
}

func tcpReachable(endpoint string) bool {
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

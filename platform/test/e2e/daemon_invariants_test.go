//go:build e2e

package e2e

import (
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestDaemonRequestIDEcho verifies the daemon always attaches a correlation id
// and echoes a caller-supplied one (sanitized), so tenant logs can be joined to
// daemon logs. This is the request-id half of the trace wiring.
func TestDaemonRequestIDEcho(t *testing.T) {
	d := startDaemon(t, daemonOpts{})

	t.Run("generated when absent", func(t *testing.T) {
		resp, _ := d.get("/healthz")
		if resp.Header.Get("X-Request-Id") == "" {
			t.Error("no X-Request-Id on response")
		}
	})

	t.Run("echoes a supplied id", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, d.baseURL+"/healthz", nil)
		req.Header.Set("X-Request-Id", "e2e-corr-123")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		_ = resp.Body.Close()
		if got := resp.Header.Get("X-Request-Id"); got != "e2e-corr-123" {
			t.Errorf("X-Request-Id = %q, want e2e-corr-123", got)
		}
	})
}

// TestDaemonTraceparentJoin verifies an inbound W3C traceparent is honored: the
// daemon joins that trace (same trace id, echoed as X-Trace-Id) instead of
// starting a new root — end-to-end correlation from an upstream caller.
func TestDaemonTraceparentJoin(t *testing.T) {
	d := startDaemon(t, daemonOpts{})
	writeGateFlow(t, d.workDir)

	const traceID = "0af7651916cd43dd8448eb211c80319c"
	req, _ := http.NewRequest(http.MethodPost,
		d.baseURL+"/validate"+q(map[string]string{"config": "runfabric.yml", "stage": "dev"}), nil)
	req.Header.Set("traceparent", "00-"+traceID+"-b7ad6b7169203331-01")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	_ = resp.Body.Close()
	if got := resp.Header.Get("X-Trace-Id"); got != traceID {
		t.Errorf("X-Trace-Id = %q, want %q (inbound traceparent must be joined)", got, traceID)
	}
}

// TestDaemonPathConfinement verifies the config path is confined to the daemon
// workspace: traversal and absolute paths are rejected rather than read off
// disk. This is a security boundary, so it is a first-class E2E.
func TestDaemonPathConfinement(t *testing.T) {
	d := startDaemon(t, daemonOpts{})
	writeGateFlow(t, d.workDir)

	for _, bad := range []string{"../../../../etc/passwd", "/etc/passwd", "../runfabric.yml"} {
		resp, _ := d.post("/validate"+q(map[string]string{"config": bad, "stage": "dev"}), nil)
		if resp.StatusCode == http.StatusOK {
			t.Errorf("config=%q was accepted (status 200); want rejection", bad)
		}
	}
}

// TestDaemonNonLoopbackBindRequiresKey verifies the daemon refuses to expose an
// unauthenticated API beyond loopback: binding a non-loopback address without
// --api-key must fail fast rather than listen.
func TestDaemonNonLoopbackBindRequiresKey(t *testing.T) {
	port := freePort(t)
	cmd := exec.Command(daemonBin, "--address", "0.0.0.0", "--port", strconv.Itoa(port))
	cmd.Dir = t.TempDir()
	out, err := cmd.CombinedOutput()
	if err == nil {
		_ = cmd.Process.Kill()
		t.Fatalf("daemon started on 0.0.0.0 without --api-key; want refusal.\noutput:\n%s", out)
	}
	if !strings.Contains(string(out), "api-key") {
		t.Errorf("refusal message did not mention api-key:\n%s", out)
	}
}

// TestDaemonAPIKeyEnforced verifies that, when --api-key is set, requests
// without the key are rejected and requests with it succeed.
func TestDaemonAPIKeyEnforced(t *testing.T) {
	d := startDaemon(t, daemonOpts{apiKey: "s3cr3t"})
	writeGateFlow(t, d.workDir)

	t.Run("missing key rejected", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost,
			d.baseURL+"/validate"+q(map[string]string{"config": "runfabric.yml", "stage": "dev"}), nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			t.Errorf("validate without X-API-Key = 200; want rejection")
		}
	})

	t.Run("present key accepted", func(t *testing.T) {
		// d.post attaches the api key (d.apiKey set).
		resp, _ := d.post("/validate"+q(map[string]string{"config": "runfabric.yml", "stage": "dev"}), nil)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("validate with X-API-Key = %d; want 200", resp.StatusCode)
		}
	})

	_ = time.Second
}

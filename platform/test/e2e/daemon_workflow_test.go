//go:build e2e

package e2e

import (
	"net/http"
	"net/url"
	"testing"
)

// q builds a workflow/config query string for the seeded workspace.
func q(params map[string]string) string {
	v := url.Values{}
	for k, val := range params {
		v.Set(k, val)
	}
	return "?" + v.Encode()
}

func TestDaemonHealthAndVersion(t *testing.T) {
	d := startDaemon(t, daemonOpts{})

	for _, path := range []string{"/healthz", "/readyz", "/version"} {
		resp, _ := d.get(path)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", path, resp.StatusCode)
		}
	}
}

func TestDaemonConfigSurface(t *testing.T) {
	d := startDaemon(t, daemonOpts{})
	writeGateFlow(t, d.workDir)

	t.Run("validate accepts a good config", func(t *testing.T) {
		resp, _ := d.post("/validate"+q(map[string]string{"config": "runfabric.yml", "stage": "dev"}), map[string]any{})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("validate status = %d, want 200", resp.StatusCode)
		}
	})

	t.Run("resolve exposes the configured workflows", func(t *testing.T) {
		resp, body := d.post("/resolve"+q(map[string]string{"config": "runfabric.yml", "stage": "dev"}), map[string]any{})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("resolve status = %d, want 200", resp.StatusCode)
		}
		// Engine resolves to the Go config: workflows under Workflows[].Name.
		wfs, _ := body["Workflows"].([]any)
		found := false
		for _, w := range wfs {
			if m, ok := w.(map[string]any); ok && m["Name"] == "gate-flow" {
				found = true
			}
		}
		if !found {
			t.Errorf("resolve did not surface workflow gate-flow; body keys=%v", keys(body))
		}
	})
}

func TestDaemonWorkflowApproveResume(t *testing.T) {
	d := startDaemon(t, daemonOpts{})
	writeGateFlow(t, d.workDir)

	// 1) run -> pauses at the human-approval gate.
	resp, body := d.post("/workflow/run"+q(map[string]string{
		"config": "runfabric.yml", "stage": "dev", "name": "gate-flow",
	}), map[string]any{})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("run status = %d, want 200 (body=%v)", resp.StatusCode, body)
	}
	run, status := runObject(body)
	if status != "paused" {
		t.Fatalf("run status = %q, want paused", status)
	}
	runID, _ := run["runId"].(string)
	if runID == "" {
		t.Fatal("run returned no runId")
	}
	if cp, _ := run["checkpoint"].(map[string]any); cp["currentStepId"] != "signoff" {
		t.Errorf("paused step = %v, want signoff", cp["currentStepId"])
	}

	// 2) approve -> resumes to completion, decision recorded on the step.
	resp, body = d.post("/workflow/approve"+q(map[string]string{
		"config": "runfabric.yml", "stage": "dev", "runId": runID,
		"decision": "approve", "reviewer": "e2e",
	}), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("approve status = %d, want 200 (body=%v)", resp.StatusCode, body)
	}
	_, status = runObject(body)
	if status != "ok" {
		t.Fatalf("resumed status = %q, want ok", status)
	}
	if got := stepDecision(body, "signoff"); got != "approve" {
		t.Errorf("signoff decision = %q, want approve", got)
	}

	// 3) status -> the run is persisted and still terminal.
	resp, body = d.post("/workflow/status"+q(map[string]string{
		"config": "runfabric.yml", "stage": "dev", "runId": runID,
	}), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if _, s := runObject(body); s != "ok" {
		t.Errorf("persisted status = %q, want ok", s)
	}
}

func TestDaemonWorkflowCancel(t *testing.T) {
	d := startDaemon(t, daemonOpts{})
	writeGateFlow(t, d.workDir)

	_, body := d.post("/workflow/run"+q(map[string]string{
		"config": "runfabric.yml", "stage": "dev", "name": "gate-flow",
	}), map[string]any{})
	run, status := runObject(body)
	if status != "paused" {
		t.Fatalf("run status = %q, want paused", status)
	}
	runID, _ := run["runId"].(string)

	// Cancel marks the run cancel-requested; a paused run stays paused (the
	// status flips to cancelled only when execution next observes the flag).
	resp, body := d.post("/workflow/cancel"+q(map[string]string{
		"config": "runfabric.yml", "stage": "dev", "runId": runID,
	}), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("cancel status = %d, want 200", resp.StatusCode)
	}
	run, _ = runObject(body)
	if req, _ := run["cancelRequested"].(bool); !req {
		t.Errorf("cancelRequested = %v, want true (run=%v)", run["cancelRequested"], keys(run))
	}
}

func TestDaemonWorkflowRunsList(t *testing.T) {
	d := startDaemon(t, daemonOpts{})
	writeGateFlow(t, d.workDir)

	for i := 0; i < 2; i++ {
		d.post("/workflow/run"+q(map[string]string{
			"config": "runfabric.yml", "stage": "dev", "name": "gate-flow",
		}), map[string]any{})
	}

	// /workflow/runs returns a JSON array (not an object) — read it raw.
	resp, raw := d.postRaw("/workflow/runs" + q(map[string]string{
		"config": "runfabric.yml", "stage": "dev", "limit": "5",
	}))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("runs status = %d, want 200", resp.StatusCode)
	}
	arr := decodeArray(t, raw)
	if len(arr) < 2 {
		t.Errorf("runs returned %d, want >= 2", len(arr))
	}
}

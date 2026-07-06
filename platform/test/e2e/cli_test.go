//go:build e2e

package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIVersion(t *testing.T) {
	res := runCLI(t, t.TempDir(), nil, "--version")
	if !res.ok() {
		t.Fatalf("runfabric --version failed: %v\n%s", res.err, res.stderr)
	}
	if strings.TrimSpace(res.stdout) == "" {
		t.Error("--version printed nothing")
	}
}

func TestCLIDoctor(t *testing.T) {
	dir := t.TempDir()
	writeGateFlow(t, dir)
	// Doctor validates config + provider readiness. Provider creds are absent,
	// so a credential check will report not-ready, but the command completes.
	res := runCLI(t, dir, nil, "doctor", "-c", "runfabric.yml")
	if !res.ok() {
		t.Fatalf("doctor exited non-zero: %v\nstdout:%s\nstderr:%s", res.err, res.stdout, res.stderr)
	}
	// The human "✔ Doctor complete." marker goes to stderr; stdout is a JSON
	// envelope. Assert the command succeeded and reported its checks.
	var env map[string]any
	if err := json.Unmarshal([]byte(res.stdout), &env); err != nil {
		t.Fatalf("doctor stdout is not JSON: %v\n%s", err, res.stdout)
	}
	if env["ok"] != true {
		t.Errorf("doctor ok = %v, want true\n%s", env["ok"], res.stdout)
	}
	if data, _ := env["data"].(map[string]any); data["checks"] == nil {
		t.Errorf("doctor reported no checks:\n%s", res.stdout)
	}
}

// TestCLIWorkflowRunApprove drives the durable workflow lifecycle entirely
// through the CLI binary: run pauses at the gate, approve resumes it to ok.
func TestCLIWorkflowRunApprove(t *testing.T) {
	dir := t.TempDir()
	writeGateFlow(t, dir)

	run := runCLI(t, dir, nil, "workflow", "run", "--name", "gate-flow", "--json")
	if !run.ok() {
		t.Fatalf("workflow run failed: %v\n%s", run.err, run.stderr)
	}
	runID, status := cliRunFields(t, run.stdout)
	if status != "paused" {
		t.Fatalf("run status = %q, want paused\n%s", status, run.stdout)
	}
	if runID == "" {
		t.Fatal("run produced no runId")
	}

	appr := runCLI(t, dir, nil, "workflow", "approve", "--run-id", runID, "--decision", "approve", "--reviewer", "e2e", "--json")
	if !appr.ok() {
		t.Fatalf("workflow approve failed: %v\n%s", appr.err, appr.stderr)
	}
	if _, s := cliRunFields(t, appr.stdout); s != "ok" {
		t.Fatalf("resumed status = %q, want ok\n%s", s, appr.stdout)
	}
}

// TestCLIInitScaffold verifies `runfabric init` scaffolds a runnable project.
func TestCLIInitScaffold(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "proj")
	res := runCLI(t, parent, nil,
		"init", "--dir", target, "--non-interactive",
		"--service", "e2e-init", "--provider", "aws-lambda")
	if !res.ok() {
		t.Fatalf("init failed: %v\nstdout:%s\nstderr:%s", res.err, res.stdout, res.stderr)
	}
	if _, err := os.Stat(filepath.Join(target, "runfabric.yml")); err != nil {
		t.Errorf("init did not scaffold runfabric.yml: %v", err)
	}
}

// TestCLITraceparentEnvDoesNotBreakCLI ensures the TRACEPARENT env join added to
// the CLI mains is inert on the happy path (a caller-supplied trace context must
// not change command behavior). Join correctness is unit-tested in telemetry.
func TestCLITraceparentEnvDoesNotBreakCLI(t *testing.T) {
	dir := t.TempDir()
	writeGateFlow(t, dir)
	env := []string{"TRACEPARENT=00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"}
	res := runCLI(t, dir, env, "doctor", "-c", "runfabric.yml")
	if !res.ok() {
		t.Fatalf("doctor with TRACEPARENT failed: %v\n%s", res.err, res.stderr)
	}
}

// cliRunFields parses {data:{run:{runId,status}}} | {run:{...}} | {...} from a
// --json workflow command envelope.
func cliRunFields(t *testing.T, stdout string) (runID, status string) {
	t.Helper()
	var env map[string]any
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("parse CLI json: %v\nstdout:%s", err, stdout)
	}
	// Unwrap the { ok, command, data } envelope when present.
	obj := env
	if data, ok := env["data"].(map[string]any); ok {
		obj = data
	}
	if res, ok := obj["result"].(map[string]any); ok {
		obj = res
	}
	run, status := runObject(obj)
	runID, _ = run["runId"].(string)
	return runID, status
}

//go:build e2e

// Package e2e holds black-box end-to-end tests that exercise the REAL
// runfabric / runfabricd / runfabricw binaries — the CLI as a subprocess and
// the daemon over its HTTP API — rather than calling app.* in-process.
//
// Run with:
//
//	make e2e            # or: go test -tags e2e ./platform/test/e2e/... -v
//
// The default lane is fully offline (control plane + durable workflows +
// cross-cutting invariants) and needs no cloud credentials. The deploy lane
// (deploy → invoke → logs → remove) runs against a Floci cloud emulator when
// one is reachable on AWS_ENDPOINT_URL (default http://localhost:4566); it
// auto-skips otherwise. See README.md.
package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// Binaries built once by TestMain; paths shared across tests.
var (
	daemonBin string
	cliBin    string
	workerBin string
	repoRoot  string
)

func TestMain(m *testing.M) {
	root, err := findRepoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: locate repo root: %v\n", err)
		os.Exit(1)
	}
	repoRoot = root

	binDir, err := os.MkdirTemp("", "rf-e2e-bin-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: temp bin dir: %v\n", err)
		os.Exit(1)
	}

	for name, pkg := range map[string]string{
		"runfabricd": "./cmd/runfabricd",
		"runfabric":  "./cmd/runfabric",
		"runfabricw": "./cmd/runfabricw",
	} {
		out := filepath.Join(binDir, name)
		build := exec.Command("go", "build", "-o", out, pkg)
		build.Dir = root
		if b, err := build.CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "e2e: build %s: %v\n%s\n", name, err, b)
			os.Exit(1)
		}
		switch name {
		case "runfabricd":
			daemonBin = out
		case "runfabric":
			cliBin = out
		case "runfabricw":
			workerBin = out
		}
	}

	code := m.Run()
	_ = os.RemoveAll(binDir)
	os.Exit(code)
}

// findRepoRoot walks up from the working directory to the module root (go.mod).
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found from %s upward", dir)
		}
		dir = parent
	}
}

// ---- daemon process ----

type daemonOpts struct {
	workDir string   // config workspace root (daemon CWD); defaults to a temp dir
	address string   // defaults to 127.0.0.1
	apiKey  string   // when set, passed as --api-key
	env     []string // extra environment (e.g. TRACEPARENT, AWS_ENDPOINT_URL)
}

type daemonProc struct {
	t       *testing.T
	baseURL string
	workDir string
	apiKey  string
	stderr  *bytes.Buffer
}

// startDaemon builds+spawns runfabricd on a free loopback port and waits for
// readiness. It registers cleanup to kill the process. A non-loopback address
// without an api key is expected to fail fast; use startDaemonExpectExit.
func startDaemon(t *testing.T, opts daemonOpts) *daemonProc {
	t.Helper()
	workDir := opts.workDir
	if workDir == "" {
		workDir = t.TempDir()
	}
	address := opts.address
	if address == "" {
		address = "127.0.0.1"
	}
	port := freePort(t)
	args := []string{"--address", address, "--port", strconv.Itoa(port)}
	if opts.apiKey != "" {
		args = append(args, "--api-key", opts.apiKey)
	}
	cmd := exec.Command(daemonBin, args...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), opts.env...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start daemon: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	d := &daemonProc{
		t:       t,
		baseURL: fmt.Sprintf("http://%s:%d", address, port),
		workDir: workDir,
		apiKey:  opts.apiKey,
		stderr:  &stderr,
	}
	d.waitHealthy()
	return d
}

func (d *daemonProc) waitHealthy() {
	d.t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		req, _ := http.NewRequest(http.MethodGet, d.baseURL+"/healthz", nil)
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	d.t.Fatalf("daemon did not become healthy at %s\nstderr:\n%s", d.baseURL, d.stderr.String())
}

// post issues a POST with an optional JSON body and returns the response +
// decoded body (nil map when the body is not a JSON object).
func (d *daemonProc) post(path string, body any) (*http.Response, map[string]any) {
	d.t.Helper()
	return d.do(http.MethodPost, path, body)
}

func (d *daemonProc) get(path string) (*http.Response, map[string]any) {
	d.t.Helper()
	return d.do(http.MethodGet, path, nil)
}

func (d *daemonProc) do(method, path string, body any) (*http.Response, map[string]any) {
	d.t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			d.t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, d.baseURL+path, reader)
	if err != nil {
		d.t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("content-type", "application/json")
	}
	if d.apiKey != "" {
		req.Header.Set("X-API-Key", d.apiKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		d.t.Fatalf("%s %s: %v", method, path, err)
	}
	raw, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return resp, out
}

// postRaw issues a POST (no body) and returns the response plus raw bytes, for
// endpoints that return a JSON array rather than an object (e.g. /workflow/runs).
func (d *daemonProc) postRaw(path string) (*http.Response, []byte) {
	d.t.Helper()
	req, err := http.NewRequest(http.MethodPost, d.baseURL+path, nil)
	if err != nil {
		d.t.Fatalf("new request: %v", err)
	}
	if d.apiKey != "" {
		req.Header.Set("X-API-Key", d.apiKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		d.t.Fatalf("POST %s: %v", path, err)
	}
	raw, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return resp, raw
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("free port: %v", err)
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port
}

// ---- CLI subprocess ----

type cliResult struct {
	stdout string
	stderr string
	err    error
}

func (r cliResult) ok() bool { return r.err == nil }

// runCLI runs the runfabric binary in workDir with extra env and returns its
// captured output. A non-zero exit is reported via cliResult.err, not t.Fatal,
// so tests can assert on failure paths.
func runCLI(t *testing.T, workDir string, env []string, args ...string) cliResult {
	t.Helper()
	cmd := exec.Command(cliBin, args...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), env...)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	return cliResult{stdout: out.String(), stderr: errb.String(), err: err}
}

// ---- workspace fixtures ----

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	abs := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(abs), err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", abs, err)
	}
}

// gateFlowYAML is a config-only workflow that runs fully offline: a code step,
// then a human-approval gate that pauses until approved.
const gateFlowYAML = `service: e2e-gate
provider:
  name: aws-lambda
  runtime: nodejs20.x
workflows:
  - name: gate-flow
    steps:
      - id: prep
        kind: code
      - id: signoff
        kind: human-approval
functions:
  - name: api
    entry: src/api.ts
    triggers:
      - type: http
        method: GET
        path: /x
`

// writeGateFlow seeds a workspace with the offline gate-flow config + a stub
// handler, returning the workspace-relative config path.
func writeGateFlow(t *testing.T, dir string) string {
	t.Helper()
	writeFile(t, dir, "runfabric.yml", gateFlowYAML)
	writeFile(t, dir, "src/api.ts", "export const handler = async () => ({ status: 200, body: 'ok' });\n")
	return "runfabric.yml"
}

// runObject digs {run:{...}} | {...} out of a workflow response and returns the
// run object plus its status string.
func runObject(body map[string]any) (map[string]any, string) {
	run := body
	if inner, ok := body["run"].(map[string]any); ok {
		run = inner
	}
	status, _ := run["status"].(string)
	return run, status
}

// stepDecision returns the recorded approval decision on the named step, "".
func stepDecision(body map[string]any, stepID string) string {
	run, _ := runObject(body)
	steps, _ := run["steps"].([]any)
	for _, s := range steps {
		m, ok := s.(map[string]any)
		if !ok || m["stepId"] != stepID {
			continue
		}
		if out, ok := m["output"].(map[string]any); ok {
			d, _ := out["decision"].(string)
			return d
		}
	}
	return ""
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func decodeArray(t *testing.T, raw []byte) []any {
	t.Helper()
	var arr []any
	if err := json.Unmarshal(raw, &arr); err != nil {
		t.Fatalf("decode array: %v (raw=%s)", err, string(raw))
	}
	return arr
}

//go:build integration
// +build integration

package external

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	infra "github.com/runfabric/runfabric/platform/extensions/infrastructure/protocol"
	extRuntime "github.com/runfabric/runfabric/platform/extensions/registry/loader/runtime"
	"gopkg.in/yaml.v3"
)

func TestCheckedInExternalPluginManifests(t *testing.T) {
	repoRoot := repoRoot(t)
	for _, manifestDir := range checkedInPluginDirs(t, repoRoot) {
		manifest, err := readPluginYAML(manifestDir)
		if err != nil {
			t.Fatalf("read manifest %s: %v", manifestDir, err)
		}
		t.Run(manifest.Kind+"-"+manifest.ID, func(t *testing.T) {
			expected := expectedManifestShape(t, manifest)
			if manifest.APIVersion != pluginAPIVersion {
				t.Fatalf("apiVersion = %q, want %q", manifest.APIVersion, pluginAPIVersion)
			}
			if manifest.PluginVer != 1 {
				t.Fatalf("pluginVersion = %d, want 1", manifest.PluginVer)
			}
			if manifest.Executable != expected.executable {
				t.Fatalf("executable = %q, want %q", manifest.Executable, expected.executable)
			}
			if !equalStrings(manifest.Capabilities, expected.capabilities) {
				t.Fatalf("capabilities = %#v, want %#v", manifest.Capabilities, expected.capabilities)
			}
			if !equalStrings(manifest.SupportsRuntime, expected.supportsRuntime) {
				t.Fatalf("supportsRuntime = %#v, want %#v", manifest.SupportsRuntime, expected.supportsRuntime)
			}
			if manifest.Permissions.FS != expected.permissions.fs || manifest.Permissions.Env != expected.permissions.env || manifest.Permissions.Network != expected.permissions.network || manifest.Permissions.Cloud != expected.permissions.cloud {
				t.Fatalf("permissions = {fs:%t env:%t network:%t cloud:%t}, want {fs:%t env:%t network:%t cloud:%t}", manifest.Permissions.FS, manifest.Permissions.Env, manifest.Permissions.Network, manifest.Permissions.Cloud, expected.permissions.fs, expected.permissions.env, expected.permissions.network, expected.permissions.cloud)
			}
		})
	}
}

func TestRealExternalPluginBinaries_Smoke(t *testing.T) {
	repoRoot := repoRoot(t)

	for _, manifestDir := range checkedInProviderManifestDirs(t, repoRoot) {
		manifest := mustReadProviderManifest(t, manifestDir)
		t.Run("provider-"+manifest.ID, func(t *testing.T) {
			if shouldSkipProviderSmoke(manifest) {
				t.Skipf("provider %s smoke skipped: standalone module not buildable in current workspace layout", manifest.ID)
			}
			relPkg, ok := providerBuildPackage(repoRoot, manifestDir)
			if !ok {
				t.Fatalf("could not locate provider main package for %s", manifest.ID)
			}
			bin := buildRealPluginBinary(t, repoRoot, relPkg, manifest.ID+"-provider")
			proc := startPluginProcess(t, bin, nil)
			defer proc.Close()

			hs := proc.handshake(t)
			if !containsStrings(hs.Capabilities, expectedHandshakeCapabilities(manifest)) {
				t.Fatalf("capabilities = %#v, want to include %#v", hs.Capabilities, expectedHandshakeCapabilities(manifest))
			}
			if len(hs.SupportsRuntime) > 0 && !containsStrings(hs.SupportsRuntime, manifest.SupportsRuntime) {
				t.Fatalf("supportsRuntime = %#v, want to include %#v", hs.SupportsRuntime, manifest.SupportsRuntime)
			}
			if len(hs.SupportsTriggers) > 0 && !containsStrings(hs.SupportsTriggers, manifest.SupportsTriggers) {
				t.Fatalf("supportsTriggers = %#v, want to include %#v", hs.SupportsTriggers, manifest.SupportsTriggers)
			}

			// Provider smoke: method wiring should exist. Backends may error without cloud creds, which is acceptable.
			var doctor map[string]any
			err := proc.call("Doctor", map[string]any{"config": map[string]any{"service": "svc"}, "stage": "dev"}, &doctor)
			if err != nil && strings.Contains(err.Error(), "method_not_found") {
				t.Fatalf("Doctor method missing: %v", err)
			}
		})
	}

	for _, manifestDir := range checkedInPluginDirsByKind(t, repoRoot, "router") {
		manifest := mustReadManifest(t, manifestDir)
		t.Run("router-"+manifest.ID, func(t *testing.T) {
			relPkg := filepath.ToSlash(filepath.Join(strings.TrimPrefix(manifestDir, repoRoot+string(filepath.Separator)), "cmd"))
			bin := buildRealPluginBinary(t, repoRoot, relPkg, manifest.ID+"-router")
			proc := startPluginProcess(t, bin, nil)
			defer proc.Close()

			hs := proc.handshake(t)
			if !equalStrings(hs.Capabilities, expectedHandshakeCapabilities(manifest)) {
				t.Fatalf("capabilities = %#v, want %#v", hs.Capabilities, expectedHandshakeCapabilities(manifest))
			}

			params, wantErr := routerSmokeRequest(t, manifest)
			var out map[string]any
			err := proc.call("Sync", params, &out)
			if err == nil || !strings.Contains(err.Error(), wantErr) {
				t.Fatalf("expected router smoke error containing %q, got %v", wantErr, err)
			}
		})
	}

	for _, manifestDir := range checkedInPluginDirsByKind(t, repoRoot, "runtime") {
		manifest := mustReadManifest(t, manifestDir)
		t.Run("runtime-"+manifest.ID, func(t *testing.T) {
			relPkg := filepath.ToSlash(filepath.Join("extensions/runtimes/cmd", manifest.ID))
			bin := buildRealPluginBinary(t, repoRoot, relPkg, manifest.ID+"-runtime")
			proc := startPluginProcess(t, bin, nil)
			defer proc.Close()

			hs := proc.handshake(t)
			if !equalStrings(hs.Capabilities, expectedHandshakeCapabilities(manifest)) {
				t.Fatalf("capabilities = %#v, want %#v", hs.Capabilities, expectedHandshakeCapabilities(manifest))
			}
			if !equalStrings(hs.SupportsRuntime, manifest.SupportsRuntime) {
				t.Fatalf("supportsRuntime = %#v, want %#v", hs.SupportsRuntime, manifest.SupportsRuntime)
			}

			assertRuntimeSmoke(t, proc, manifest)
		})
	}

	for _, manifestDir := range checkedInPluginDirsByKind(t, repoRoot, "simulator") {
		manifest := mustReadManifest(t, manifestDir)
		t.Run("simulator-"+manifest.ID, func(t *testing.T) {
			relPkg := filepath.ToSlash(filepath.Join(strings.TrimPrefix(manifestDir, repoRoot+string(filepath.Separator)), "cmd"))
			bin := buildRealPluginBinary(t, repoRoot, relPkg, manifest.ID+"-simulator")
			proc := startPluginProcess(t, bin, nil)
			defer proc.Close()

			hs := proc.handshake(t)
			if !equalStrings(hs.Capabilities, expectedHandshakeCapabilities(manifest)) {
				t.Fatalf("capabilities = %#v, want %#v", hs.Capabilities, expectedHandshakeCapabilities(manifest))
			}

			assertSimulatorSmoke(t, proc, manifest)
		})
	}

	for _, manifestDir := range checkedInPluginDirsByKind(t, repoRoot, "state") {
		manifest := mustReadManifest(t, manifestDir)
		t.Run("state-"+manifest.ID, func(t *testing.T) {
			relPkg := filepath.ToSlash(filepath.Join("extensions/states/cmd", manifest.ID))
			bin := buildRealPluginBinary(t, repoRoot, relPkg, manifest.ID+"-state")

			if startupErr := expectedStateStartupError(manifest.ID); startupErr != "" {
				proc := startPluginProcess(t, bin, []string{"RUNFABRIC_STATE_ROOT=" + t.TempDir()})
				defer proc.Close()
				var hs infra.Handshake
				err := proc.call("Handshake", nil, &hs)
				if err == nil || !strings.Contains(err.Error(), startupErr) {
					t.Fatalf("expected startup error containing %q, got %v", startupErr, err)
				}
				return
			}

			root := t.TempDir()
			env := []string{"RUNFABRIC_STATE_ROOT=" + root}
			if manifest.ID == "sqlite" {
				env = append(env, "RUNFABRIC_STATE_SQLITE_PATH="+filepath.Join(root, "state.db"))
			}
			proc := startPluginProcess(t, bin, env)
			defer proc.Close()

			hs := proc.handshake(t)
			if !equalStrings(hs.Capabilities, expectedHandshakeCapabilities(manifest)) {
				t.Fatalf("capabilities = %#v, want %#v", hs.Capabilities, expectedHandshakeCapabilities(manifest))
			}

			assertStateSmoke(t, proc, manifest)
		})
	}
}

type pluginProcess struct {
	t      *testing.T
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	scan   *bufio.Scanner
	stderr *bytes.Buffer
	seq    int
}

func startPluginProcess(t *testing.T, executable string, extraEnv []string) *pluginProcess {
	t.Helper()
	cmd := exec.Command(executable)
	cmd.Env = append(os.Environ(), extraEnv...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start plugin %s: %v", executable, err)
	}
	scan := bufio.NewScanner(stdout)
	scan.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	return &pluginProcess{t: t, cmd: cmd, stdin: stdin, stdout: stdout, scan: scan, stderr: stderr}
}

func (p *pluginProcess) Close() {
	if p == nil || p.cmd == nil {
		return
	}
	_ = p.stdin.Close()
	_ = p.stdout.Close()
	if p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
	_, _ = p.cmd.Process.Wait()
}

func (p *pluginProcess) handshake(t *testing.T) infra.Handshake {
	t.Helper()
	var hs infra.Handshake
	if err := p.call("Handshake", nil, &hs); err != nil {
		t.Fatalf("Handshake error: %v", err)
	}
	if hs.ProtocolVersion != extRuntime.ProtocolVersion {
		t.Fatalf("protocolVersion = %q, want %q", hs.ProtocolVersion, extRuntime.ProtocolVersion)
	}
	if hs.Platform == "" {
		t.Fatal("expected non-empty platform")
	}
	return hs
}

func (p *pluginProcess) call(method string, params any, out any) error {
	p.seq++
	req := Request{ID: fmt.Sprintf("%d", p.seq), Method: method, ProtocolVersion: extRuntime.ProtocolVersion, Params: params}
	if err := json.NewEncoder(p.stdin).Encode(req); err != nil {
		return fmt.Errorf("write request: %w", err)
	}
	if !p.scan.Scan() {
		if err := p.scan.Err(); err != nil {
			return fmt.Errorf("read response: %w (stderr: %s)", err, strings.TrimSpace(p.stderr.String()))
		}
		return fmt.Errorf("plugin produced no response (stderr: %s)", strings.TrimSpace(p.stderr.String()))
	}
	var resp Response
	if err := json.Unmarshal(p.scan.Bytes(), &resp); err != nil {
		return fmt.Errorf("decode response: %w (line: %s)", err, string(p.scan.Bytes()))
	}
	if resp.Error != nil {
		if resp.Error.Code != "" {
			return fmt.Errorf("%s: %s", resp.Error.Code, resp.Error.Message)
		}
		return fmt.Errorf("%s", resp.Error.Message)
	}
	if out == nil {
		return nil
	}
	blob, err := json.Marshal(resp.Result)
	if err != nil {
		return fmt.Errorf("marshal response result: %w", err)
	}
	if len(blob) == 0 || string(blob) == "null" {
		return nil
	}
	return json.Unmarshal(blob, out)
}

func buildRealPluginBinary(t *testing.T, repoRoot, relPkg, binaryName string) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), binaryName)
	cmd := exec.Command("go", "build", "-o", out, "./"+filepath.ToSlash(relPkg))
	cmd.Dir = repoRoot
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build %s: %v\n%s", relPkg, err, string(b))
	}
	if err := os.Chmod(out, 0o755); err != nil {
		t.Fatalf("chmod %s: %v", out, err)
	}
	return out
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve repo root: runtime caller unavailable")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for idx := range got {
		if got[idx] != want[idx] {
			return false
		}
	}
	return true
}

func containsStrings(got, want []string) bool {
	if len(want) == 0 {
		return true
	}
	seen := make(map[string]struct{}, len(got))
	for _, value := range got {
		seen[value] = struct{}{}
	}
	for _, value := range want {
		if _, ok := seen[value]; !ok {
			return false
		}
	}
	return true
}

type expectedManifest struct {
	executable      string
	capabilities    []string
	supportsRuntime []string
	permissions     struct {
		fs      bool
		env     bool
		network bool
		cloud   bool
	}
}

func checkedInPluginDirs(t *testing.T, repoRoot string) []string {
	t.Helper()
	patterns := []string{
		"extensions/routers/*/plugin.yaml",
		"extensions/runtimes/*/plugin.yaml",
		"extensions/simulators/plugin.yaml",
		"extensions/states/*/plugin.yaml",
	}
	var dirs []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(repoRoot, pattern))
		if err != nil {
			t.Fatalf("glob %s: %v", pattern, err)
		}
		for _, match := range matches {
			dirs = append(dirs, filepath.Dir(match))
		}
	}
	sort.Strings(dirs)
	return dirs
}

func checkedInPluginDirsByKind(t *testing.T, repoRoot, kind string) []string {
	t.Helper()
	all := checkedInPluginDirs(t, repoRoot)
	out := make([]string, 0, len(all))
	for _, dir := range all {
		manifest := mustReadManifest(t, dir)
		if manifest.Kind == kind {
			out = append(out, dir)
		}
	}
	return out
}

func mustReadManifest(t *testing.T, dir string) *pluginYAML {
	t.Helper()
	manifest, err := readPluginYAML(dir)
	if err != nil {
		t.Fatalf("read manifest %s: %v", dir, err)
	}
	return manifest
}

func mustReadProviderManifest(t *testing.T, dir string) *pluginYAML {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "plugin.yaml"))
	if err != nil {
		t.Fatalf("read manifest %s: %v", dir, err)
	}
	var manifest pluginYAML
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("parse manifest %s: %v", dir, err)
	}
	manifest.Kind = strings.TrimSpace(manifest.Kind)
	manifest.ID = strings.TrimSpace(manifest.ID)
	if manifest.Kind == "" || manifest.ID == "" {
		t.Fatalf("provider manifest %s missing kind/id", dir)
	}
	return &manifest
}

func expectedManifestShape(t *testing.T, manifest *pluginYAML) expectedManifest {
	t.Helper()
	if manifest == nil {
		t.Fatal("manifest is required")
	}
	var out expectedManifest
	switch manifest.Kind {
	case "router":
		out.executable = "./bin/" + manifest.ID + "-router"
		out.capabilities = []string{"sync"}
		out.permissions.env = true
		out.permissions.network = true
		out.permissions.cloud = true
		out.permissions.fs = manifest.ID == "cloudflare"
	case "runtime":
		out.executable = "./bin/" + manifest.ID + "-runtime"
		out.capabilities = []string{"build", "invoke"}
		out.supportsRuntime = []string{manifest.ID}
		out.permissions.fs = true
		out.permissions.env = true
	case "simulator":
		out.executable = "./bin/" + manifest.ID + "-simulator"
		out.capabilities = []string{"simulate"}
		out.permissions.fs = true
		out.permissions.env = true
	case "state":
		out.executable = "./bin/" + manifest.ID + "-state"
		out.capabilities = []string{"state:" + manifest.ID, "backend:" + manifest.ID}
		out.permissions.fs = true
		out.permissions.env = true
		switch manifest.ID {
		case "postgres":
			out.permissions.network = true
		case "dynamodb", "s3":
			out.permissions.network = true
			out.permissions.cloud = true
		}
	default:
		t.Fatalf("unexpected plugin kind %q", manifest.Kind)
	}
	return out
}

func expectedHandshakeCapabilities(manifest *pluginYAML) []string {
	if manifest == nil {
		return nil
	}
	switch manifest.Kind {
	case "state":
		caps := []string{"lock", "journal", "receipt"}
		caps = append(caps, manifest.Capabilities...)
		return caps
	default:
		return append([]string(nil), manifest.Capabilities...)
	}
}

func checkedInProviderManifestDirs(t *testing.T, repoRoot string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(repoRoot, "extensions/providers/*/plugin.yaml"))
	if err != nil {
		t.Fatalf("glob providers manifests: %v", err)
	}
	dirs := make([]string, 0, len(matches))
	for _, match := range matches {
		dirs = append(dirs, filepath.Dir(match))
	}
	sort.Strings(dirs)
	return dirs
}

func providerBuildPackage(repoRoot, manifestDir string) (string, bool) {
	rel := strings.TrimPrefix(manifestDir, repoRoot+string(filepath.Separator))
	cmdMain := filepath.Join(manifestDir, "cmd", "main.go")
	if _, err := os.Stat(cmdMain); err == nil {
		return filepath.ToSlash(filepath.Join(rel, "cmd")), true
	}
	rootMain := filepath.Join(manifestDir, "main.go")
	if _, err := os.Stat(rootMain); err == nil {
		return filepath.ToSlash(rel), true
	}
	return "", false
}

func shouldSkipProviderSmoke(manifest *pluginYAML) bool {
	if manifest == nil {
		return false
	}
	switch manifest.ID {
	case "linode":
		return true
	default:
		return false
	}
}

func assertRuntimeSmoke(t *testing.T, proc *pluginProcess, manifest *pluginYAML) {
	t.Helper()
	root := t.TempDir()
	var handlerFile, handlerRef, runtimeLabel string
	switch manifest.ID {
	case "nodejs":
		handlerFile = "index.js"
		handlerRef = "index.handler"
		runtimeLabel = "nodejs18.x"
		if err := os.WriteFile(filepath.Join(root, handlerFile), []byte("exports.handler = async () => ({ ok: true });\n"), 0o644); err != nil {
			t.Fatalf("write handler: %v", err)
		}
	case "python":
		handlerFile = "handler.py"
		handlerRef = "handler.main"
		runtimeLabel = "python3.11"
		if err := os.WriteFile(filepath.Join(root, handlerFile), []byte("def main(event, context):\n    return {\"ok\": True}\n"), 0o644); err != nil {
			t.Fatalf("write handler: %v", err)
		}
	default:
		t.Fatalf("unexpected runtime manifest id %q", manifest.ID)
	}

	var artifact struct {
		Function   string `json:"function"`
		Runtime    string `json:"runtime"`
		OutputPath string `json:"outputPath"`
	}
	if err := proc.call("Build", map[string]any{
		"root":         root,
		"functionName": "hello",
		"function": map[string]any{
			"handler": handlerRef,
			"runtime": runtimeLabel,
		},
		"configSignature": "cfg-1",
	}, &artifact); err != nil {
		t.Fatalf("Build error: %v", err)
	}
	if artifact.Function != "hello" {
		t.Fatalf("artifact function = %q, want hello", artifact.Function)
	}
	if artifact.Runtime == "" || artifact.OutputPath == "" {
		t.Fatalf("unexpected artifact: %#v", artifact)
	}
	if _, err := os.Stat(artifact.OutputPath); err != nil {
		t.Fatalf("expected built artifact at %q: %v", artifact.OutputPath, err)
	}
}

func expectedStateStartupError(id string) string {
	switch id {
	case "postgres":
		return "RUNFABRIC_STATE_POSTGRES_DSN is required"
	case "dynamodb":
		return "RUNFABRIC_STATE_DYNAMODB_TABLE is required"
	case "s3":
		return "RUNFABRIC_STATE_S3_BUCKET is required"
	default:
		return ""
	}
}

func assertStateSmoke(t *testing.T, proc *pluginProcess, manifest *pluginYAML) {
	t.Helper()
	if manifest.ID != "local" && manifest.ID != "sqlite" {
		return
	}
	if err := proc.call("ReceiptSave", map[string]any{
		"receipt": map[string]any{
			"version":      2,
			"service":      "svc",
			"stage":        "dev",
			"provider":     "local",
			"deploymentId": "dep-1",
			"outputs":      map[string]string{"url": "https://example.com"},
			"updatedAt":    "2026-03-30T00:00:00Z",
		},
	}, nil); err != nil {
		t.Fatalf("ReceiptSave error: %v", err)
	}

	var releases []struct {
		Stage     string `json:"stage"`
		UpdatedAt string `json:"updatedAt"`
	}
	if err := proc.call("ReceiptListReleases", map[string]any{}, &releases); err != nil {
		t.Fatalf("ReceiptListReleases error: %v", err)
	}
	if len(releases) == 0 || releases[0].Stage != "dev" {
		t.Fatalf("unexpected releases: %#v", releases)
	}
}

func routerSmokeRequest(t *testing.T, manifest *pluginYAML) (map[string]any, string) {
	t.Helper()
	params := map[string]any{
		"routing": map[string]any{
			"contract": "runfabric.fabric.routing.v1",
			"service":  "svc",
			"stage":    "dev",
			"hostname": "svc.example.com",
			"strategy": "round-robin",
			"endpoints": []map[string]any{{
				"name": "primary",
				"url":  "https://example.com",
			}},
		},
		"dryRun": true,
	}
	switch manifest.ID {
	case "cloudflare":
		params["zoneID"] = "zone-1"
		return params, "router API token is required"
	case "route53":
		return params, "ROUTE53 hosted zone ID is required"
	case "ns1":
		params["zoneID"] = "example.com"
		return params, "NS1 API token is required"
	case "azure-traffic-manager":
		params["zoneID"] = "/subscriptions/test/resourceGroups/rg/providers/Microsoft.Network/trafficManagerProfiles/example"
		return params, "Azure access token is required"
	default:
		t.Fatalf("unexpected router manifest id %q", manifest.ID)
		return nil, ""
	}
}

func assertSimulatorSmoke(t *testing.T, proc *pluginProcess, manifest *pluginYAML) {
	t.Helper()
	var result struct {
		StatusCode int               `json:"statusCode"`
		Headers    map[string]string `json:"headers"`
		Body       json.RawMessage   `json:"body"`
	}
	if err := proc.call("Simulate", map[string]any{
		"service":  "svc",
		"stage":    "dev",
		"function": "hello",
		"method":   "GET",
		"path":     "/health",
	}, &result); err != nil {
		t.Fatalf("Simulate error: %v", err)
	}
	if result.StatusCode != 200 {
		t.Fatalf("statusCode = %d, want 200", result.StatusCode)
	}
	var body map[string]any
	if err := json.Unmarshal(result.Body, &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	switch manifest.ID {
	case "local":
		if body["message"] != "invoke local" {
			t.Fatalf("body message = %#v, want invoke local", body["message"])
		}
	default:
		t.Fatalf("unexpected simulator manifest id %q", manifest.ID)
	}
}

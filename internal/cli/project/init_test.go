package project

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/runfabric/runfabric/internal/cli/common"
)

func TestYAMLQuoted(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"safe alphanumeric", "my-service", "my-service"},
		{"safe with hyphen", "my-service-1", "my-service-1"},
		{"safe with underscore", "my_service", "my_service"},
		{"safe with dot", "svc.v1", "svc.v1"},
		{"empty", "", `""`},
		{"newline injection", "ok\nmalicious: key: value", `"ok\nmalicious: key: value"`},
		{"colon injection", "name: injected", `"name: injected"`},
		{"quote in value", `say "hello"`, `"say \"hello\""`},
		{"backslash", `path\to\file`, `"path\\to\\file"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := yamlQuoted(tt.in)
			if got != tt.want {
				t.Errorf("yamlQuoted(%q) = %q, want %q", tt.in, got, tt.want)
			}
			if strings.Contains(tt.in, "\n") && !strings.HasPrefix(got, `"`) {
				t.Errorf("yamlQuoted(%q) must quote string containing newline, got %q", tt.in, got)
			}
		})
	}
}

func TestInit_ValidLang(t *testing.T) {
	dir := t.TempDir()
	opts := &common.GlobalOptions{}
	cmd := newInitCmd(opts)
	cmd.SetArgs([]string{"--dir", dir, "--lang", "ts", "--no-interactive"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("init --lang ts should succeed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "runfabric.yml")); err != nil {
		t.Errorf("runfabric.yml not created: %v", err)
	}
}

func TestInit_InvalidLang(t *testing.T) {
	dir := t.TempDir()
	opts := &common.GlobalOptions{}
	cmd := newInitCmd(opts)
	cmd.SetArgs([]string{"--dir", dir, "--lang", "rust", "--no-interactive"})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil {
		t.Error("init --lang rust should fail")
	}
}

func TestInit_ProviderTemplateMatrixRejection(t *testing.T) {
	dir := t.TempDir()
	opts := &common.GlobalOptions{}
	cmd := newInitCmd(opts)
	// fly-machines does not support cron per Trigger Capability Matrix
	cmd.SetArgs([]string{"--dir", dir, "--provider", "fly-machines", "--template", "cron", "--no-interactive"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil {
		t.Error("init provider=fly-machines template=cron should fail (matrix)")
	}
}

func TestInit_ProviderTemplateMatrixAccept(t *testing.T) {
	dir := t.TempDir()
	opts := &common.GlobalOptions{}
	cmd := newInitCmd(opts)
	cmd.SetArgs([]string{"--dir", dir, "--provider", "aws-lambda", "--template", "http", "--lang", "go", "--no-interactive"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("init provider=aws-lambda template=http lang=go should succeed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "handler.go")); err != nil {
		t.Errorf("handler.go not created: %v", err)
	}
}

func TestInit_RejectsLegacyTemplateAliases(t *testing.T) {
	for _, legacy := range []string{"api", "worker"} {
		t.Run(legacy, func(t *testing.T) {
			dir := t.TempDir()
			opts := &common.GlobalOptions{}
			cmd := newInitCmd(opts)
			cmd.SetArgs([]string{"--dir", dir, "--provider", "aws-lambda", "--template", legacy, "--lang", "go", "--no-interactive"})
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})
			err := cmd.Execute()
			if err == nil {
				t.Fatalf("expected %q template alias to be rejected", legacy)
			}
			if !strings.Contains(err.Error(), "is no longer supported") {
				t.Fatalf("expected explicit legacy template error, got %v", err)
			}
		})
	}
}

func TestProviderComment(t *testing.T) {
	tests := []struct {
		provider string
		wantSub  string
	}{
		{"aws-lambda", "aws-lambda"},
		{"gcp-functions", "gcp-functions"},
		{"fly-machines", "fly-machines"},
		{"cloudflare-workers", "cloudflare-workers"},
		{"unknown-provider", ""},
	}
	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			got := providerComment(tt.provider)
			if tt.wantSub != "" && !strings.Contains(got, tt.wantSub) {
				t.Errorf("providerComment(%q) = %q, want substring %q", tt.provider, got, tt.wantSub)
			}
		})
	}
}

func TestGeneratePackageJSON_DepsAndScripts(t *testing.T) {
	o := &initOpts{
		Lang:            "ts",
		Service:         "ux-test",
		Provider:        "aws-lambda",
		CallLocal:       true,
		WithBuildScript: true,
	}

	raw := generatePackageJSON(o)
	var pkg map[string]any
	if err := json.Unmarshal([]byte(raw), &pkg); err != nil {
		t.Fatalf("package json must be valid: %v", err)
	}

	deps, _ := pkg["dependencies"].(map[string]any)
	// Runtime dependencies are optional; scaffold has empty deps for immediate npm install
	if len(deps) != 0 {
		t.Fatalf("expected empty runtime dependencies in scaffold, got %v", deps)
	}

	scripts, _ := pkg["scripts"].(map[string]any)
	if got := scripts["call:local"]; got != "concurrently npm:build:watch 'runfabric invoke local -c runfabric.yml --serve --watch'" {
		t.Fatalf("expected call:local script, got %v", got)
	}
	if got := scripts["build"]; got != "tsc" {
		t.Fatalf("expected build script, got %v", got)
	}
	if got := scripts["build:watch"]; got != "tsc --watch --preserveWatchOutput" {
		t.Fatalf("expected build:watch script, got %v", got)
	}

	devDeps, _ := pkg["devDependencies"].(map[string]any)
	if devDeps["typescript"] != "^5.0.0" {
		t.Fatalf("expected typescript devDependency, got %v", devDeps)
	}
	if devDeps["concurrently"] != "^9.0.1" {
		t.Fatalf("expected concurrently devDependency, got %v", devDeps)
	}
}

func TestGeneratePackageJSON_TSBuildOptional(t *testing.T) {
	withoutBuild := &initOpts{Lang: "ts", Service: "ts-no-build", Provider: "aws-lambda"}
	withBuild := &initOpts{Lang: "ts", Service: "ts-build", Provider: "aws-lambda", WithBuildScript: true}

	var pkgWithout map[string]any
	if err := json.Unmarshal([]byte(generatePackageJSON(withoutBuild)), &pkgWithout); err != nil {
		t.Fatalf("unmarshal without build: %v", err)
	}
	if _, ok := pkgWithout["devDependencies"]; ok {
		t.Fatalf("did not expect devDependencies when --with-build is false")
	}
	if scripts, ok := pkgWithout["scripts"].(map[string]any); ok {
		if _, hasBuild := scripts["build"]; hasBuild {
			t.Fatalf("did not expect build script when --with-build is false")
		}
		if _, hasBuildWatch := scripts["build:watch"]; hasBuildWatch {
			t.Fatalf("did not expect build:watch script when --with-build is false")
		}
	}

	var pkgWith map[string]any
	if err := json.Unmarshal([]byte(generatePackageJSON(withBuild)), &pkgWith); err != nil {
		t.Fatalf("unmarshal with build: %v", err)
	}
	if _, ok := pkgWith["devDependencies"]; !ok {
		t.Fatalf("expected devDependencies when --with-build is true")
	}
	if scripts, ok := pkgWith["scripts"].(map[string]any); ok {
		if _, hasBuild := scripts["build"]; !hasBuild {
			t.Fatalf("expected build script when --with-build is true")
		}
		if _, hasBuildWatch := scripts["build:watch"]; !hasBuildWatch {
			t.Fatalf("expected build:watch script when --with-build is true")
		}
	}
	if devDeps, ok := pkgWith["devDependencies"].(map[string]any); ok {
		if _, hasConcurrently := devDeps["concurrently"]; !hasConcurrently {
			t.Fatalf("expected concurrently devDependency when --with-build is true")
		}
	}
}

func TestGeneratePackageJSON_JSCallLocalScriptUnchanged(t *testing.T) {
	o := &initOpts{Lang: "js", Service: "js-svc", CallLocal: true}

	var pkg map[string]any
	if err := json.Unmarshal([]byte(generatePackageJSON(o)), &pkg); err != nil {
		t.Fatalf("unmarshal js package json: %v", err)
	}
	scripts, _ := pkg["scripts"].(map[string]any)
	if got := scripts["call:local"]; got != "runfabric invoke local -c runfabric.yml --serve --watch" {
		t.Fatalf("expected js call:local script unchanged, got %v", got)
	}
}

func TestInit_TypeScriptScaffoldIncludesBuildToolingByDefault(t *testing.T) {
	dir := t.TempDir()
	opts := &common.GlobalOptions{}
	cmd := newInitCmd(opts)
	cmd.SetArgs([]string{"--dir", dir, "--lang", "ts", "--no-interactive", "--skip-install"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("init ts scaffold: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		t.Fatalf("read package.json: %v", err)
	}
	var pkg map[string]any
	if err := json.Unmarshal(raw, &pkg); err != nil {
		t.Fatalf("unmarshal package.json: %v", err)
	}
	scripts, _ := pkg["scripts"].(map[string]any)
	if got := scripts["build"]; got != "tsc" {
		t.Fatalf("expected build script by default, got %v", got)
	}
	if _, ok := pkg["devDependencies"].(map[string]any); !ok {
		t.Fatalf("expected devDependencies by default for ts scaffold")
	}
	if _, err := os.Stat(filepath.Join(dir, "tsconfig.json")); err != nil {
		t.Fatalf("tsconfig.json not created: %v", err)
	}
}

func TestInit_TypeScriptScaffoldBuildsDistHandlerAfterInstall(t *testing.T) {
	dir := t.TempDir()
	binDir := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "pm.log")
	npmPath := filepath.Join(binDir, "npm")
	npmScript := "#!/bin/sh\n" +
		"set -eu\n" +
		"printf '%s\\n' \"$*\" >> \"$RUNFABRIC_TEST_PM_LOG\"\n" +
		"cmd=${1:-}\n" +
		"case \"$cmd\" in\n" +
		"  install)\n" +
		"    mkdir -p node_modules\n" +
		"    ;;\n" +
		"  run)\n" +
		"    if [ \"${2:-}\" != \"build\" ]; then\n" +
		"      echo \"unexpected npm run target: ${2:-}\" >&2\n" +
		"      exit 1\n" +
		"    fi\n" +
		"    mkdir -p dist\n" +
		"    cat > dist/handler.js <<'EOF'\n" +
		"exports.handler = async () => ({ statusCode: 200, body: JSON.stringify({ ok: true }) });\n" +
		"EOF\n" +
		"    ;;\n" +
		"  *)\n" +
		"    echo \"unexpected npm command: $*\" >&2\n" +
		"    exit 1\n" +
		"    ;;\n" +
		"esac\n"
	if err := os.WriteFile(npmPath, []byte(npmScript), 0o755); err != nil {
		t.Fatalf("write fake npm: %v", err)
	}

	t.Setenv("RUNFABRIC_TEST_PM_LOG", logPath)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	opts := &common.GlobalOptions{}
	cmd := newInitCmd(opts)
	cmd.SetArgs([]string{"--dir", dir, "--lang", "ts", "--no-interactive"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("init ts scaffold with install/build: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "dist", "handler.js")); err != nil {
		t.Fatalf("dist/handler.js not created after install/build: %v", err)
	}

	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read package manager log: %v", err)
	}
	log := strings.TrimSpace(string(logBytes))
	if !strings.Contains(log, "install") {
		t.Fatalf("expected install command in log, got %q", log)
	}
	if !strings.Contains(log, "run build") {
		t.Fatalf("expected build command in log, got %q", log)
	}
	if strings.Index(log, "install") > strings.Index(log, "run build") {
		t.Fatalf("expected install before build, got log %q", log)
	}
}

func TestInitBinary_TypeScriptScaffoldBuildsDistHandlerAfterInstall(t *testing.T) {
	dir := t.TempDir()
	binDir := t.TempDir()
	pmLogPath := filepath.Join(t.TempDir(), "pm.log")
	npmPath := filepath.Join(binDir, "npm")
	npmScript := "#!/bin/sh\n" +
		"set -eu\n" +
		"printf '%s\\n' \"$*\" >> \"$RUNFABRIC_TEST_PM_LOG\"\n" +
		"cmd=${1:-}\n" +
		"case \"$cmd\" in\n" +
		"  install)\n" +
		"    mkdir -p node_modules\n" +
		"    ;;\n" +
		"  run)\n" +
		"    if [ \"${2:-}\" != \"build\" ]; then\n" +
		"      echo \"unexpected npm run target: ${2:-}\" >&2\n" +
		"      exit 1\n" +
		"    fi\n" +
		"    mkdir -p dist\n" +
		"    cat > dist/handler.js <<'EOF'\n" +
		"exports.handler = async () => ({ statusCode: 200, body: JSON.stringify({ ok: true }) });\n" +
		"EOF\n" +
		"    ;;\n" +
		"  *)\n" +
		"    echo \"unexpected npm command: $*\" >&2\n" +
		"    exit 1\n" +
		"    ;;\n" +
		"esac\n"
	if err := os.WriteFile(npmPath, []byte(npmScript), 0o755); err != nil {
		t.Fatalf("write fake npm: %v", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot := filepath.Clean(filepath.Join(wd, "..", "..", ".."))
	binaryPath := filepath.Join(t.TempDir(), "runfabric")

	buildCmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/runfabric")
	buildCmd.Dir = repoRoot
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("build runfabric binary: %v\n%s", err, string(out))
	}

	cmd := exec.Command(binaryPath,
		"init",
		"--dir", dir,
		"--lang", "ts",
		"--no-interactive",
	)
	cmd.Env = append(os.Environ(),
		"RUNFABRIC_TEST_PM_LOG="+pmLogPath,
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("runfabric init via binary: %v\n%s", err, string(out))
	}

	if _, err := os.Stat(filepath.Join(dir, "dist", "handler.js")); err != nil {
		t.Fatalf("dist/handler.js not created after binary init: %v", err)
	}

	logBytes, err := os.ReadFile(pmLogPath)
	if err != nil {
		t.Fatalf("read package manager log: %v", err)
	}
	log := strings.TrimSpace(string(logBytes))
	if !strings.Contains(log, "install") {
		t.Fatalf("expected install command in log, got %q", log)
	}
	if !strings.Contains(log, "run build") {
		t.Fatalf("expected build command in log, got %q", log)
	}
	if strings.Index(log, "install") > strings.Index(log, "run build") {
		t.Fatalf("expected install before build, got log %q", log)
	}
}

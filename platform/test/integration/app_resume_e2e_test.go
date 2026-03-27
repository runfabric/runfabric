package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/runfabric/runfabric/platform/workflow/app"
	"github.com/runfabric/runfabric/platform/workflow/recovery"
)

func TestAppResumeE2EIfEnabled(t *testing.T) {
	if os.Getenv("RUNFABRIC_AWS_INTEGRATION") != "1" {
		t.Skip("set RUNFABRIC_AWS_INTEGRATION=1 to enable real AWS integration test")
	}

	tmp := t.TempDir()

	srcDir := filepath.Join(tmp, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}

	handlerPath := filepath.Join(srcDir, "handler.js")
	code := `
exports.hello = async () => ({ statusCode: 200, body: JSON.stringify({ ok: true }) });
`
	if err := os.WriteFile(handlerPath, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(tmp, "runfabric.yml")
	cfgContent := `
service: rf-phase16-test
provider:
  name: aws
  runtime: nodejs20.x
  region: ap-southeast-1
functions:
  hello:
    handler: src/handler.hello
    events:
      - http:
          path: /hello
          method: get
`
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o644); err != nil {
		t.Fatal(err)
	}

	_ = os.Setenv("RUNFABRIC_FAULT_ENABLED", "1")
	_ = os.Setenv("RUNFABRIC_FAIL_AFTER_PHASE", "ensure_http_api")

	_, err := app.Deploy(cfgPath, "dev", "", false, false, nil, "")
	if err == nil {
		t.Fatal("expected injected failure during deploy")
	}

	_ = os.Unsetenv("RUNFABRIC_FAULT_ENABLED")
	_ = os.Unsetenv("RUNFABRIC_FAIL_AFTER_PHASE")

	result, err := app.Recover(cfgPath, "dev", recovery.ModeResume)
	if err != nil {
		t.Fatal(err)
	}

	m, ok := result.(*map[string]any)
	if !ok {
		t.Fatalf("unexpected recovery result type")
	}

	if (*m)["recovered"] != true {
		t.Fatalf("expected recovered=true, got %#v", *m)
	}
}

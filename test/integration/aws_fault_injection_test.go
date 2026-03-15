package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/runfabric/runfabric/internal/config"
	awsprovider "github.com/runfabric/runfabric/providers/aws"
)

func TestAWSFaultInjectionAndResumeIfEnabled(t *testing.T) {
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
service: rf-phase15-test
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

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err = config.Resolve(cfg, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if err := config.Validate(cfg); err != nil {
		t.Fatal(err)
	}

	_ = os.Setenv("RUNFABRIC_FAULT_ENABLED", "1")
	_ = os.Setenv("RUNFABRIC_FAIL_AFTER_PHASE", "ensure_http_api")
	defer func() {
		_ = os.Unsetenv("RUNFABRIC_FAULT_ENABLED")
		_ = os.Unsetenv("RUNFABRIC_FAIL_AFTER_PHASE")
	}()

	p := awsprovider.New()
	_, err = p.Deploy(cfg, "dev", tmp)
	if err == nil {
		t.Fatal("expected injected deploy failure")
	}

	// Real resume should be invoked through app recovery flow in a fuller integration path.
	// Here we only validate that the failed deploy produced a recoverable state on disk.
	journalPath := filepath.Join(tmp, ".runfabric", "journals", "rf-phase15-test-dev.journal.json")
	if _, err := os.Stat(journalPath); err != nil {
		t.Fatalf("expected journal after injected failure: %v", err)
	}
}

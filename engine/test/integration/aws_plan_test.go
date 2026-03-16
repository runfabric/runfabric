package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/runfabric/runfabric/engine/internal/config"
	awsprovider "github.com/runfabric/runfabric/engine/providers/aws"
)

func TestRealAWSDeployIfEnabled(t *testing.T) {
	if os.Getenv("RUNFABRIC_AWS_INTEGRATION") != "1" {
		t.Skip("set RUNFABRIC_AWS_INTEGRATION=1 to enable real AWS integration test")
	}

	tmp := t.TempDir()

	srcDir := filepath.Join(tmp, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}

	handlerPath := filepath.Join(srcDir, "handler.js")
	if err := os.WriteFile(handlerPath, []byte(`exports.hello = async () => ({ statusCode: 200, body: JSON.stringify({ ok: true }) });`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(tmp, "runfabric.yml")
	cfgContent := `
service: rf-phase5-test
provider:
  name: aws
  runtime: nodejs20.x
  region: ap-southeast-1
functions:
  hello:
    handler: src/handler.hello
    memory: 128
    timeout: 10
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

	p := awsprovider.New()

	deployResult, err := p.Deploy(cfg, "dev", tmp)
	if err != nil {
		t.Fatal(err)
	}

	if deployResult.Outputs["hello"] == "" {
		t.Fatal("expected hello function URL")
	}

	_, err = p.Invoke(cfg, "dev", "hello", []byte(`{"name":"test"}`))
	if err != nil {
		t.Fatal(err)
	}

	_, err = p.Logs(cfg, "dev", "hello")
	if err != nil {
		t.Fatal(err)
	}

	_, err = p.Remove(cfg, "dev", tmp)
	if err != nil {
		t.Fatal(err)
	}
}

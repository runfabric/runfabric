package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/runfabric/runfabric/engine/internal/config"
	awsprovider "github.com/runfabric/runfabric/engine/providers/aws"
)

func TestRealAWSReconcileIfEnabled(t *testing.T) {
	if os.Getenv("RUNFABRIC_AWS_INTEGRATION") != "1" {
		t.Skip("set RUNFABRIC_AWS_INTEGRATION=1 to enable real AWS integration test")
	}

	tmp := t.TempDir()

	srcDir := filepath.Join(tmp, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}

	handlerPath := filepath.Join(srcDir, "handler.js")
	handlerCode := `
exports.hello = async () => ({ statusCode: 200, body: JSON.stringify({ ok: true }) });
exports.goodbye = async () => ({ statusCode: 200, body: JSON.stringify({ bye: true }) });
`
	if err := os.WriteFile(handlerPath, []byte(handlerCode), 0o644); err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(tmp, "runfabric.yml")
	cfgContent1 := `
service: rf-phase7-test
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
  goodbye:
    handler: src/handler.goodbye
    events:
      - http:
          path: /bye
          method: get
`
	if err := os.WriteFile(cfgPath, []byte(cfgContent1), 0o644); err != nil {
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

	_, err = p.Deploy(cfg, "dev", tmp)
	if err != nil {
		t.Fatal(err)
	}

	cfgContent2 := `
service: rf-phase7-test
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
	if err := os.WriteFile(cfgPath, []byte(cfgContent2), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg2, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	cfg2, err = config.Resolve(cfg2, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if err := config.Validate(cfg2); err != nil {
		t.Fatal(err)
	}

	planResult, err := p.Plan(cfg2, "dev", cfgContent2)
	if err != nil {
		t.Fatal(err)
	}
	if len(planResult.Plan.Actions) == 0 {
		t.Fatal("expected plan actions after drift change")
	}

	_, err = p.Deploy(cfg2, "dev", tmp)
	if err != nil {
		t.Fatal(err)
	}

	_, err = p.Remove(cfg2, "dev", tmp)
	if err != nil {
		t.Fatal(err)
	}
}

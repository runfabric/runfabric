package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	provider "github.com/runfabric/runfabric/platform/core/contracts/provider"
	"github.com/runfabric/runfabric/platform/core/model/config"
)

func TestRealAWSHTTPDeployIfEnabled(t *testing.T) {
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
exports.createUser = async () => ({ statusCode: 201, body: JSON.stringify({ created: true }) });
`
	if err := os.WriteFile(handlerPath, []byte(handlerCode), 0o644); err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(tmp, "runfabric.yml")
	cfgContent := `
service: rf-phase6-test
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
  createUser:
    handler: src/handler.createUser
    events:
      - http:
          path: /users
          method: post
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

	p := resolveAWSProvider(t)

	planResult, err := p.Plan(context.Background(), provider.PlanRequest{Config: cfg, Stage: "dev", Root: cfgContent})
	if err != nil {
		t.Fatal(err)
	}
	if len(planResult.Plan.Actions) == 0 {
		t.Fatal("expected plan actions")
	}

	deployResult, err := p.Deploy(context.Background(), provider.DeployRequest{Config: cfg, Stage: "dev", Root: tmp})
	if err != nil {
		t.Fatal(err)
	}

	if deployResult.Outputs["hello[0]"] == "" {
		t.Fatal("expected hello route URL")
	}
	if deployResult.Outputs["createUser[0]"] == "" {
		t.Fatal("expected createUser route URL")
	}

	_, err = p.Remove(context.Background(), provider.RemoveRequest{Config: cfg, Stage: "dev", Root: tmp})
	if err != nil {
		t.Fatal(err)
	}
}

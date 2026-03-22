package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	provider "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	"github.com/runfabric/runfabric/platform/core/model/config"
	awsprovider "github.com/runfabric/runfabric/platform/extensions/interfaces/providers/aws"
)

func TestRealAWSRemoteDriftDetectionIfEnabled(t *testing.T) {
	if os.Getenv("RUNFABRIC_AWS_INTEGRATION") != "1" {
		t.Skip("set RUNFABRIC_AWS_INTEGRATION=1 to enable real AWS integration test")
	}

	tmp := t.TempDir()

	srcDir := filepath.Join(tmp, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}

	handlerPath := filepath.Join(srcDir, "handler.js")
	codeV1 := `
exports.hello = async () => ({ statusCode: 200, body: JSON.stringify({ version: "v1" }) });
`
	if err := os.WriteFile(handlerPath, []byte(codeV1), 0o644); err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(tmp, "runfabric.yml")
	cfgContent := `
service: rf-phase9-test
provider:
  name: aws
  runtime: nodejs20.x
  region: ap-southeast-1
functions:
  hello:
    handler: src/handler.hello
    memory: 128
    timeout: 10
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

	p := awsprovider.New()

	_, err = p.Deploy(context.Background(), provider.DeployRequest{Config: cfg, Stage: "dev", Root: tmp})
	if err != nil {
		t.Fatal(err)
	}

	codeV2 := `
exports.hello = async () => ({ statusCode: 200, body: JSON.stringify({ version: "v2" }) });
`
	if err := os.WriteFile(handlerPath, []byte(codeV2), 0o644); err != nil {
		t.Fatal(err)
	}

	planResult, err := p.Plan(context.Background(), provider.PlanRequest{Config: cfg, Stage: "dev", Root: tmp})
	if err != nil {
		t.Fatal(err)
	}

	foundUpdate := false
	for _, action := range planResult.Plan.Actions {
		if action.Resource == "lambda" && action.Type == "update" {
			foundUpdate = true
			break
		}
	}

	if !foundUpdate {
		t.Fatal("expected lambda update action after code drift")
	}

	_, err = p.Remove(context.Background(), provider.RemoveRequest{Config: cfg, Stage: "dev", Root: tmp})
	if err != nil {
		t.Fatal(err)
	}
}

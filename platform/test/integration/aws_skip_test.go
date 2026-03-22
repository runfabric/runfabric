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

func TestRealAWSSkipIfUnchanged(t *testing.T) {
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
service: rf-phase8-test
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

	first, err := p.Deploy(context.Background(), provider.DeployRequest{Config: cfg, Stage: "dev", Root: tmp})
	if err != nil {
		t.Fatal(err)
	}
	if first.Outputs["hello[0]"] == "" {
		t.Fatal("expected route output on first deploy")
	}

	second, err := p.Deploy(context.Background(), provider.DeployRequest{Config: cfg, Stage: "dev", Root: tmp})
	if err != nil {
		t.Fatal(err)
	}

	if second.Metadata["lambda:hello:operation"] != "skipped" {
		t.Fatalf("expected skipped second deploy, got %q", second.Metadata["lambda:hello:operation"])
	}

	_, err = p.Remove(context.Background(), provider.RemoveRequest{Config: cfg, Stage: "dev", Root: tmp})
	if err != nil {
		t.Fatal(err)
	}
}

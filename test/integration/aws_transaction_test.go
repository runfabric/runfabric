package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/runfabric/runfabric/internal/config"
	awsprovider "github.com/runfabric/runfabric/providers/aws"
)

func TestRealAWSDeployLockIfEnabled(t *testing.T) {
	if os.Getenv("RUNFABRIC_AWS_INTEGRATION") != "1" {
		t.Skip("set RUNFABRIC_AWS_INTEGRATION=1 to enable real AWS integration test")
	}

	tmp := t.TempDir()

	srcDir := filepath.Join(tmp, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}

	handlerPath := filepath.Join(srcDir, "handler.js")
	if err := os.WriteFile(handlerPath, []byte(`exports.hello = async () => ({ statusCode: 200, body: "ok" });`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(tmp, "runfabric.yml")
	cfgContent := `
service: rf-phase10-test
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

	p := awsprovider.New()

	out, err := p.Deploy(cfg, "dev", tmp)
	if err != nil {
		t.Fatal(err)
	}

	if out.Metadata["deploy:transactional"] != "true" {
		t.Fatal("expected transactional deploy metadata")
	}

	_, err = p.Remove(cfg, "dev", tmp)
	if err != nil {
		t.Fatal(err)
	}
}

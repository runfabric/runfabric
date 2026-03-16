package unit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/runfabric/runfabric/engine/internal/config"
)

func TestBackendConfigLoad(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "runfabric.yml")

	content := `
service: hello-api
provider:
  name: aws
  runtime: nodejs20.x
  region: ap-southeast-1
backend:
  kind: aws-remote
  s3Bucket: my-bucket
  s3Prefix: apps/dev
  lockTable: rf-locks
functions:
  hello:
    handler: src/handler.hello
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err = config.Resolve(cfg, "dev")
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Backend == nil || cfg.Backend.Kind != "aws-remote" {
		t.Fatal("expected backend kind aws-remote")
	}
}

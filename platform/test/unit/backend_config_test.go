package unit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/runfabric/runfabric/platform/core/model/config"
)

func TestBackendConfigLoad(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "runfabric.yml")

	content := "service: hello-api\n" +
		"provider:\n" +
		"  name: aws-lambda\n" +
		"  runtime: nodejs20.x\n" +
		"  region: ap-southeast-1\n" +
		"backend:\n" +
		"  kind: s3\n" +
		"  s3Bucket: my-bucket\n" +
		"  s3Prefix: apps/dev\n" +
		"  lockTable: rf-locks\n" +
		"functions:\n" +
		"  - name: hello\n" +
		"    entry: src/handler.hello\n"
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

	if cfg.Backend == nil || cfg.Backend.Kind != "s3" {
		t.Fatal("expected backend kind s3")
	}
}

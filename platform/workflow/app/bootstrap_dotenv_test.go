package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBootstrap_LoadsDotEnvForEnvSecretResolution(t *testing.T) {
	providerName, runtimeName := testProviderNameAndRuntime(t)

	project := t.TempDir()
	configPath := filepath.Join(project, "runfabric.yml")
	configYAML := []byte(`service: svc
provider:
  name: ` + providerName + `
  runtime: ` + runtimeName + `
secrets:
  db_password: "${env:DB_PASSWORD}"
functions:
  - name: api
    entry: src/handler.default
    env:
      DB_PASSWORD: "${secret:db_password}"
`)
	if err := os.WriteFile(configPath, configYAML, 0o644); err != nil {
		t.Fatalf("write runfabric.yml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(project, ".env"), []byte("DB_PASSWORD=from-dotenv\n"), 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	if err := os.Unsetenv("DB_PASSWORD"); err != nil {
		t.Fatalf("unset DB_PASSWORD: %v", err)
	}

	ctx, err := Bootstrap(configPath, "dev", "")
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if got := ctx.Config.Functions["api"].Environment["DB_PASSWORD"]; got != "from-dotenv" {
		t.Fatalf("DB_PASSWORD=%q want from-dotenv", got)
	}
}

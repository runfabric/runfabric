package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/extensions/providers"
)

type flyRunner struct{}

func (r *flyRunner) Deploy(ctx context.Context, cfg *config.Config, stage, root string) (*providers.DeployResult, error) {
	appName := fmt.Sprintf("%s-%s", cfg.Service, stage)
	flyToml := fmt.Sprintf(`app = "%s"

[build]

[env]

[http_service]
  internal_port = 8080
  force_https = true
  auto_stop_machines = true
  auto_start_machines = true
  processes = ["app"]
  [http_service.concurrency]
    type = "requests"
    hard_limit = 25
    soft_limit = 20
`, appName)
	tomlPath := filepath.Join(root, "fly.toml")
	if _, err := os.Stat(tomlPath); os.IsNotExist(err) {
		if err := os.WriteFile(tomlPath, []byte(flyToml), 0o644); err != nil {
			return nil, fmt.Errorf("write fly.toml: %w", err)
		}
	}
	stdout, stderr, err := runCmd(ctx, root, "fly", "deploy", "--yes")
	if err != nil {
		return nil, fmt.Errorf("fly deploy: %w\nstderr: %s", err, stderr)
	}
	url := extractURL(stdout + stderr)
	if url == "" {
		url = fmt.Sprintf("https://%s.fly.dev", appName)
	}
	artifacts := make([]providers.Artifact, 0, len(cfg.Functions))
	for fnName, fn := range cfg.Functions {
		rt := fn.Runtime
		if rt == "" {
			rt = cfg.Provider.Runtime
		}
		artifacts = append(artifacts, providers.Artifact{Function: fnName, Runtime: rt})
	}
	return &providers.DeployResult{
		Provider:     "fly-machines",
		DeploymentID: fmt.Sprintf("fly-%s-%d", stage, time.Now().Unix()),
		Outputs:      map[string]string{"url": url, "stdout": strings.TrimSpace(stdout)},
		Artifacts:    artifacts,
		Metadata:     map[string]string{"app": appName},
	}, nil
}

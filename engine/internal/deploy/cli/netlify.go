package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/extensions/providers"
)

type netlifyRunner struct{}

func (r *netlifyRunner) Deploy(ctx context.Context, cfg *config.Config, stage, root string) (*providers.DeployResult, error) {
	args := []string{"netlify", "deploy", "--build", "--dir=.", "--message=runfabric"}
	if stage == "prod" || stage == "production" {
		args = append(args, "--prod")
	}
	stdout, stderr, err := runCmd(ctx, root, "npx", args...)
	if err != nil {
		return nil, fmt.Errorf("netlify deploy: %w\nstderr: %s", err, stderr)
	}
	url := extractURL(stdout + stderr)
	if url == "" {
		url = "https://" + cfg.Service + ".netlify.app"
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
		Provider:     "netlify",
		DeploymentID: fmt.Sprintf("netlify-%s-%d", stage, time.Now().Unix()),
		Outputs:      map[string]string{"url": url, "stdout": strings.TrimSpace(stdout)},
		Artifacts:    artifacts,
	}, nil
}

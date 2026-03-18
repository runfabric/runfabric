package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/extensions/providers"
)

type vercelRunner struct{}

func (r *vercelRunner) Deploy(ctx context.Context, cfg *config.Config, stage, root string) (*providers.DeployResult, error) {
	// Vercel CLI: vercel --yes (prod: vercel --prod)
	args := []string{"vercel", "--yes"}
	if stage == "prod" || stage == "production" {
		args = append(args, "--prod")
	}
	stdout, stderr, err := runCmd(ctx, root, "npx", args...)
	if err != nil {
		return nil, fmt.Errorf("vercel deploy: %w\nstderr: %s", err, stderr)
	}
	url := extractURL(stdout + stderr)
	if url == "" {
		url = "https://" + cfg.Service + ".vercel.app"
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
		Provider:     "vercel",
		DeploymentID: fmt.Sprintf("vercel-%s-%d", stage, time.Now().Unix()),
		Outputs:      map[string]string{"url": url, "stdout": strings.TrimSpace(stdout)},
		Artifacts:    artifacts,
		Metadata:     map[string]string{"project": cfg.Service},
	}, nil
}

package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/extensions/providers"
)

type digitalOceanRunner struct{}

func (r *digitalOceanRunner) Deploy(ctx context.Context, cfg *config.Config, stage, root string) (*providers.DeployResult, error) {
	// doctl serverless deploy . or npm run deploy if project has it
	stdout, stderr, err := runCmd(ctx, root, "doctl", "serverless", "deploy", ".", "--remote-build")
	if err != nil {
		return nil, fmt.Errorf("doctl serverless deploy: %w\nstderr: %s", err, stderr)
	}
	url := extractURL(stdout + stderr)
	artifacts := make([]providers.Artifact, 0, len(cfg.Functions))
	for fnName, fn := range cfg.Functions {
		rt := fn.Runtime
		if rt == "" {
			rt = cfg.Provider.Runtime
		}
		artifacts = append(artifacts, providers.Artifact{Function: fnName, Runtime: rt})
	}
	return &providers.DeployResult{
		Provider:     "digitalocean-functions",
		DeploymentID: fmt.Sprintf("do-%s-%d", stage, time.Now().Unix()),
		Outputs:      map[string]string{"url": url, "stdout": strings.TrimSpace(stdout)},
		Artifacts:    artifacts,
	}, nil
}

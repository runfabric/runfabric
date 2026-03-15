package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/runfabric/runfabric/internal/config"
	"github.com/runfabric/runfabric/internal/providers"
)

type alibabaRunner struct{}

func (r *alibabaRunner) Deploy(ctx context.Context, cfg *config.Config, stage, root string) (*providers.DeployResult, error) {
	// Alibaba FC: fun deploy or s deploy (Serverless Devs)
	stdout, stderr, err := runCmd(ctx, root, "s", "deploy", "-y")
	if err != nil {
		// Fallback: try fun deploy if s not found
		stdout, stderr, err = runCmd(ctx, root, "fun", "deploy", "-y")
		if err != nil {
			return nil, fmt.Errorf("alibaba deploy (s/fun): %w\nstderr: %s", err, stderr)
		}
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
		Provider:     "alibaba-fc",
		DeploymentID: fmt.Sprintf("alibaba-%s-%d", stage, time.Now().Unix()),
		Outputs:      map[string]string{"url": url, "stdout": strings.TrimSpace(stdout)},
		Artifacts:    artifacts,
	}, nil
}

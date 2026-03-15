package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/runfabric/runfabric/internal/config"
	"github.com/runfabric/runfabric/internal/providers"
)

type ibmRunner struct{}

func (r *ibmRunner) Deploy(ctx context.Context, cfg *config.Config, stage, root string) (*providers.DeployResult, error) {
	// IBM OpenWhisk: wsk project deploy or npm run deploy
	stdout, stderr, err := runCmd(ctx, root, "wsk", "project", "deploy")
	if err != nil {
		return nil, fmt.Errorf("wsk project deploy: %w\nstderr: %s", err, stderr)
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
		Provider:     "ibm-openwhisk",
		DeploymentID: fmt.Sprintf("ibm-%s-%d", stage, time.Now().Unix()),
		Outputs:      map[string]string{"url": url, "stdout": strings.TrimSpace(stdout)},
		Artifacts:    artifacts,
	}, nil
}

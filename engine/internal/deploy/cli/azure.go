package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/providers"
)

type azureRunner struct{}

func (r *azureRunner) Deploy(ctx context.Context, cfg *config.Config, stage, root string) (*providers.DeployResult, error) {
	appName := fmt.Sprintf("%s-%s", cfg.Service, stage)
	// func azure functionapp publish <appName>
	stdout, stderr, err := runCmd(ctx, root, "func", "azure", "functionapp", "publish", appName)
	if err != nil {
		return nil, fmt.Errorf("func azure functionapp publish: %w\nstderr: %s", err, stderr)
	}
	url := extractURL(stdout + stderr)
	if url == "" {
		url = "https://" + appName + ".azurewebsites.net"
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
		Provider:     "azure-functions",
		DeploymentID: fmt.Sprintf("azure-%s-%d", stage, time.Now().Unix()),
		Outputs:      map[string]string{"url": url, "stdout": strings.TrimSpace(stdout)},
		Artifacts:    artifacts,
		Metadata:     map[string]string{"functionApp": appName},
	}, nil
}

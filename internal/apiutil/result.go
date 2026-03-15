package apiutil

import (
	"fmt"
	"time"

	"github.com/runfabric/runfabric/internal/config"
	"github.com/runfabric/runfabric/internal/providers"
)

// BuildDeployResult returns a base DeployResult for the given provider, config, and stage.
func BuildDeployResult(provider string, cfg *config.Config, stage string) *providers.DeployResult {
	artifacts := make([]providers.Artifact, 0, len(cfg.Functions))
	for fnName, fn := range cfg.Functions {
		rt := fn.Runtime
		if rt == "" {
			rt = cfg.Provider.Runtime
		}
		artifacts = append(artifacts, providers.Artifact{Function: fnName, Runtime: rt})
	}
	return &providers.DeployResult{
		Provider:     provider,
		DeploymentID: fmt.Sprintf("%s-%s-%d", provider, stage, time.Now().Unix()),
		Outputs:      make(map[string]string),
		Artifacts:    artifacts,
		Metadata:     make(map[string]string),
	}
}

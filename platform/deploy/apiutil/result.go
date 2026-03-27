package apiutil

import (
	"fmt"
	"time"

	providers "github.com/runfabric/runfabric/internal/provider/contracts"
	sdkbridge "github.com/runfabric/runfabric/internal/provider/sdkbridge"
	"github.com/runfabric/runfabric/platform/core/model/config"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// BuildDeployResult returns a base DeployResult for the given provider, config, and stage.
func BuildDeployResult(provider string, cfg *config.Config, stage string) *providers.DeployResult {
	artifacts := make([]providers.Artifact, 0, len(cfg.Functions))
	functions := make(map[string]providers.DeployedFunction, len(cfg.Functions))
	for fnName, fn := range cfg.Functions {
		rt := fn.Runtime
		if rt == "" {
			rt = cfg.Provider.Runtime
		}
		artifacts = append(artifacts, providers.Artifact{Function: fnName, Runtime: rt})
		functions[fnName] = providers.DeployedFunction{ResourceName: fnName}
	}
	return &providers.DeployResult{
		Provider:     provider,
		DeploymentID: fmt.Sprintf("%s-%s-%d", provider, stage, time.Now().Unix()),
		Outputs:      make(map[string]string),
		Artifacts:    artifacts,
		Metadata:     make(map[string]string),
		Functions:    functions,
	}
}

// BuildSDKDeployResult is now in internal/provider/sdkbridge; re-export from canonical location.
func BuildSDKDeployResult(providerID string, cfg sdkprovider.Config, stage string) *sdkprovider.DeployResult {
	return sdkbridge.BuildSDKDeployResult(providerID, cfg, stage)
}

package apiutil

import (
	"fmt"
	"time"

	sdkbridge "github.com/runfabric/runfabric/internal/provider/sdkbridge"
	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
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

// BuildSDKDeployResult returns a base sdkprovider.DeployResult for API-dispatch providers.
// cfg is the raw SDK config (map[string]any); decoded for artifact and function metadata.
func BuildSDKDeployResult(providerID string, cfg sdkprovider.Config, stage string) *sdkprovider.DeployResult {
	coreCfg, _ := sdkbridge.ToCoreConfig(cfg)
	out := &sdkprovider.DeployResult{
		Provider:     providerID,
		DeploymentID: fmt.Sprintf("%s-%s-%d", providerID, stage, time.Now().Unix()),
		Outputs:      make(map[string]string),
		Metadata:     make(map[string]string),
	}
	if coreCfg != nil {
		out.Artifacts = make([]sdkprovider.Artifact, 0, len(coreCfg.Functions))
		out.Functions = make(map[string]sdkprovider.DeployedFunction, len(coreCfg.Functions))
		for fnName, fn := range coreCfg.Functions {
			rt := fn.Runtime
			if rt == "" {
				rt = coreCfg.Provider.Runtime
			}
			out.Artifacts = append(out.Artifacts, sdkprovider.Artifact{Function: fnName, Runtime: rt})
			out.Functions[fnName] = sdkprovider.DeployedFunction{ResourceName: fnName}
		}
	}
	return out
}

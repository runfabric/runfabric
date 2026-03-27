package sdkbridge

import (
	"fmt"
	"time"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// BuildSDKDeployResult constructs a populated DeployResult from the provider ID,
// SDK config, and stage name. It is a convenience helper used by API-based providers.
func BuildSDKDeployResult(providerID string, cfg sdkprovider.Config, stage string) *sdkprovider.DeployResult {
	coreCfg, _ := ToCoreConfig(cfg)
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
			runtime := fn.Runtime
			if runtime == "" {
				runtime = coreCfg.Provider.Runtime
			}
			out.Artifacts = append(out.Artifacts, sdkprovider.Artifact{Function: fnName, Runtime: runtime})
			out.Functions[fnName] = sdkprovider.DeployedFunction{ResourceName: fnName}
		}
	}
	return out
}

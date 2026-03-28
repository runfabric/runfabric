package codec

import (
	"fmt"
	"time"

	providers "github.com/runfabric/runfabric/internal/provider/contracts"
)

// BuildDeployResult constructs a populated DeployResult from provider ID,
// transport-safe config, and stage name.
func BuildDeployResult(providerID string, cfg providers.Config, stage string) *providers.DeployResult {
	coreCfg, _ := ToCoreConfig(cfg)
	out := &providers.DeployResult{
		Provider:     providerID,
		DeploymentID: fmt.Sprintf("%s-%s-%d", providerID, stage, time.Now().Unix()),
		Outputs:      make(map[string]string),
		Metadata:     make(map[string]string),
	}
	if coreCfg != nil {
		out.Artifacts = make([]providers.Artifact, 0, len(coreCfg.Functions))
		out.Functions = make(map[string]providers.DeployedFunction, len(coreCfg.Functions))
		for fnName, fn := range coreCfg.Functions {
			runtime := fn.Runtime
			if runtime == "" {
				runtime = coreCfg.Provider.Runtime
			}
			out.Artifacts = append(out.Artifacts, providers.Artifact{Function: fnName, Runtime: runtime})
			out.Functions[fnName] = providers.DeployedFunction{ResourceName: fnName}
		}
	}
	return out
}

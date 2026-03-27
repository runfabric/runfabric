package exec

import (
	providers "github.com/runfabric/runfabric/internal/provider/contracts"
	"github.com/runfabric/runfabric/platform/core/model/config"
	state "github.com/runfabric/runfabric/platform/core/state/core"
	planner "github.com/runfabric/runfabric/platform/planner/engine"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

type Context struct {
	Root      string
	Config    *config.Config
	Stage     string
	Artifacts map[string]sdkprovider.Artifact
	Desired   *planner.DesiredState
	Actual    *planner.ActualState
	Receipt   *state.Receipt

	Outputs  map[string]string
	Metadata map[string]string

	Faults FaultConfig

	Result *providers.DeployResult
}

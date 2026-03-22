package exec

import (
	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	"github.com/runfabric/runfabric/platform/core/model/config"
	planner "github.com/runfabric/runfabric/platform/core/planner/engine"
	state "github.com/runfabric/runfabric/platform/core/state/core"
)

type Context struct {
	Root      string
	Config    *config.Config
	Stage     string
	Artifacts map[string]providers.Artifact
	Desired   *planner.DesiredState
	Actual    *planner.ActualState
	Receipt   *state.Receipt

	Outputs  map[string]string
	Metadata map[string]string

	Faults FaultConfig

	Result *providers.DeployResult
}

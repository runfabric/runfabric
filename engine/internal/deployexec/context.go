package deployexec

import (
	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/extensions/providers"
	"github.com/runfabric/runfabric/engine/internal/planner"
	"github.com/runfabric/runfabric/engine/internal/state"
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

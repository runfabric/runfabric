package lifecycle

import (
	"github.com/runfabric/runfabric/internal/config"
	"github.com/runfabric/runfabric/internal/providers"
)

func Plan(reg *providers.Registry, cfg *config.Config, stage, root string) (*providers.PlanResult, error) {
	p, err := reg.Get(cfg.Provider.Name)
	if err != nil {
		return nil, err
	}
	return p.Plan(cfg, stage, root)
}

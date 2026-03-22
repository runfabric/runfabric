package lifecycle

import (
	"context"

	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	"github.com/runfabric/runfabric/platform/core/model/config"
)

func Plan(reg *providers.Registry, cfg *config.Config, stage, root string) (*providers.PlanResult, error) {
	p, ok := reg.Get(cfg.Provider.Name)
	if !ok {
		return nil, providers.ErrProviderNotFound(cfg.Provider.Name)
	}
	return p.Plan(context.Background(), providers.PlanRequest{Config: cfg, Stage: stage, Root: root})
}

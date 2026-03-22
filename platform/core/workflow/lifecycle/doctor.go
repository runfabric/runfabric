package lifecycle

import (
	"context"

	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	"github.com/runfabric/runfabric/platform/core/model/config"
)

func Doctor(reg *providers.Registry, cfg *config.Config, stage string) (*providers.DoctorResult, error) {
	p, ok := reg.Get(cfg.Provider.Name)
	if !ok {
		return nil, providers.ErrProviderNotFound(cfg.Provider.Name)
	}
	return p.Doctor(context.Background(), providers.DoctorRequest{Config: cfg, Stage: stage})
}

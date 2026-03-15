package lifecycle

import (
	"github.com/runfabric/runfabric/internal/config"
	"github.com/runfabric/runfabric/internal/providers"
)

func Doctor(reg *providers.Registry, cfg *config.Config, stage string) (*providers.DoctorResult, error) {
	p, err := reg.Get(cfg.Provider.Name)
	if err != nil {
		return nil, err
	}
	return p.Doctor(cfg, stage)
}

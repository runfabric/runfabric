package lifecycle

import (
	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/extensions/providers"
	"github.com/runfabric/runfabric/engine/internal/state"
)

func Remove(reg *providers.Registry, cfg *config.Config, stage, root string) (*providers.RemoveResult, error) {
	p, err := reg.Get(cfg.Provider.Name)
	if err != nil {
		return nil, err
	}

	result, err := p.Remove(cfg, stage, root)
	if err != nil {
		return nil, err
	}

	if result.Removed {
		if err := state.Delete(root, stage); err != nil {
			return nil, err
		}
	}

	return result, nil
}

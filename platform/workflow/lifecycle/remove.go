package lifecycle

import (
	"context"

	providers "github.com/runfabric/runfabric/platform/core/contracts/provider"
	"github.com/runfabric/runfabric/platform/core/model/config"
	state "github.com/runfabric/runfabric/platform/core/state/core"
)

func Remove(reg *providers.Registry, cfg *config.Config, stage, root string) (*providers.RemoveResult, error) {
	p, ok := reg.Get(cfg.Provider.Name)
	if !ok {
		return nil, providers.ErrProviderNotFound(cfg.Provider.Name)
	}

	result, err := p.Remove(context.Background(), providers.RemoveRequest{Config: cfg, Stage: stage, Root: root})
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

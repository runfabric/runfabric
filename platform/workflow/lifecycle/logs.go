package lifecycle

import (
	"context"

	providers "github.com/runfabric/runfabric/platform/core/contracts/provider"
	"github.com/runfabric/runfabric/platform/core/model/config"
	appErrs "github.com/runfabric/runfabric/platform/core/model/errors"
)

func Logs(reg *providers.Registry, cfg *config.Config, stage, function string) (*providers.LogsResult, error) {
	p, ok := reg.Get(cfg.Provider.Name)
	if !ok {
		return nil, providers.ErrProviderNotFound(cfg.Provider.Name)
	}

	result, err := p.Logs(context.Background(), providers.LogsRequest{Config: cfg, Stage: stage, Function: function})
	if err != nil {
		return nil, appErrs.Wrap(appErrs.CodeLogsFailed, "logs failed", err)
	}
	return result, nil
}

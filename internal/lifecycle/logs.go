package lifecycle

import (
	"github.com/runfabric/runfabric/internal/config"
	appErrs "github.com/runfabric/runfabric/internal/errors"
	"github.com/runfabric/runfabric/internal/providers"
)

func Logs(reg *providers.Registry, cfg *config.Config, stage, function string) (*providers.LogsResult, error) {
	p, err := reg.Get(cfg.Provider.Name)
	if err != nil {
		return nil, err
	}

	result, err := p.Logs(cfg, stage, function)
	if err != nil {
		return nil, appErrs.Wrap(appErrs.CodeLogsFailed, "logs failed", err)
	}
	return result, nil
}

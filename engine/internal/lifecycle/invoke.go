package lifecycle

import (
	"github.com/runfabric/runfabric/engine/internal/config"
	appErrs "github.com/runfabric/runfabric/engine/internal/errors"
	"github.com/runfabric/runfabric/engine/internal/extensions/providers"
)

func Invoke(reg *providers.Registry, cfg *config.Config, stage, function string, payload []byte) (*providers.InvokeResult, error) {
	p, err := reg.Get(cfg.Provider.Name)
	if err != nil {
		return nil, err
	}

	result, err := p.Invoke(cfg, stage, function, payload)
	if err != nil {
		return nil, appErrs.Wrap(appErrs.CodeInvokeFailed, "invoke failed", err)
	}
	return result, nil
}

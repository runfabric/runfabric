package lifecycle

import (
	"context"

	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	"github.com/runfabric/runfabric/platform/core/model/config"
	appErrs "github.com/runfabric/runfabric/platform/core/model/errors"
)

func Invoke(reg *providers.Registry, cfg *config.Config, stage, function string, payload []byte) (*providers.InvokeResult, error) {
	p, ok := reg.Get(cfg.Provider.Name)
	if !ok {
		return nil, providers.ErrProviderNotFound(cfg.Provider.Name)
	}

	result, err := p.Invoke(context.Background(), providers.InvokeRequest{Config: cfg, Stage: stage, Function: function, Payload: payload})
	if err != nil {
		return nil, appErrs.Wrap(appErrs.CodeInvokeFailed, "invoke failed", err)
	}
	return result, nil
}

package aws

import (
	"github.com/runfabric/runfabric/engine/internal/config"
	appErrs "github.com/runfabric/runfabric/engine/internal/errors"
	"github.com/runfabric/runfabric/engine/internal/providers"
)

func (p *Provider) Deploy(cfg *config.Config, stage, root string) (*providers.DeployResult, error) {
	return nil, appErrs.Wrap(
		appErrs.CodeDeployFailed,
		"direct provider deploy is no longer supported; use the control-plane deploy path",
		nil,
	)
}

func artifactsFromMap(m map[string]providers.Artifact) []providers.Artifact {
	out := make([]providers.Artifact, 0, len(m))
	for _, a := range m {
		out = append(out, a)
	}
	return out
}

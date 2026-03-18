package aws

import (
	"context"

	"github.com/runfabric/runfabric/engine/internal/config"
)

func ensureCustomDomain(ctx context.Context, clients *AWSClients, apiID string, stageCfg *config.StageHTTPConfig) error {
	if stageCfg == nil || stageCfg.Domain == nil {
		return nil
	}

	// honest version:
	// real domain wiring requires domain name creation, API mapping, certificate setup, and likely Route53.
	// do not fake this as "done".
	return nil
}

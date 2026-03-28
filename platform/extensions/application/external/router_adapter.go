package external

import (
	"context"
	"strings"

	providers "github.com/runfabric/runfabric/internal/provider/contracts"
	routercontracts "github.com/runfabric/runfabric/platform/core/contracts/router"
)

// ExternalRouterAdapter implements routercontracts.Router by reusing the
// external plugin stdio transport used by provider adapters.
type ExternalRouterAdapter struct {
	id     string
	meta   routercontracts.PluginMeta
	client *ExternalProviderAdapter
}

func NewExternalRouterAdapter(id, executable string, meta routercontracts.PluginMeta) *ExternalRouterAdapter {
	normalizedID := strings.TrimSpace(id)
	if strings.TrimSpace(meta.ID) == "" {
		meta.ID = normalizedID
	}
	clientMeta := providers.ProviderMeta{Name: normalizedID}
	return &ExternalRouterAdapter{
		id:     normalizedID,
		meta:   meta,
		client: NewExternalProviderAdapter(normalizedID, executable, clientMeta),
	}
}

func (r *ExternalRouterAdapter) Meta() routercontracts.PluginMeta {
	meta := r.meta
	if strings.TrimSpace(meta.ID) == "" {
		meta.ID = strings.TrimSpace(r.id)
	}
	return meta
}

func (r *ExternalRouterAdapter) Sync(ctx context.Context, req routercontracts.SyncRequest) (*routercontracts.SyncResult, error) {
	_ = ctx // External adapter protocol is request/response over stdio and currently does not carry cancellation.
	var out routercontracts.SyncResult
	err := r.client.call("Sync", map[string]any{
		"routing":   req.Routing,
		"zoneID":    req.ZoneID,
		"accountID": req.AccountID,
		"dryRun":    req.DryRun,
	}, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

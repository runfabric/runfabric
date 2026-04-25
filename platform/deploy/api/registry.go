package api

import (
	"context"

	providers "github.com/runfabric/runfabric/internal/provider/contracts"
	"github.com/runfabric/runfabric/platform/core/model/config"
	state "github.com/runfabric/runfabric/platform/core/state/core"
	extdeploy "github.com/runfabric/runfabric/platform/extensions/bridge"
)

// Provider is the API-dispatch provider contract consumed by deploy/core/api.
type Provider interface {
	Deploy(ctx context.Context, cfg *config.Config, stage, root string) (*providers.DeployResult, error)
	Remove(ctx context.Context, cfg *config.Config, stage, root string, receipt *state.Receipt) (*providers.RemoveResult, error)
	Invoke(ctx context.Context, cfg *config.Config, stage, function string, payload []byte, receipt *state.Receipt) (*providers.InvokeResult, error)
	Logs(ctx context.Context, cfg *config.Config, stage, function string, receipt *state.Receipt) (*providers.LogsResult, error)
}

func getProvider(name string) (Provider, bool) {
	return extdeploy.GetProvider(name)
}

func hasProvider(name string) bool {
	return extdeploy.HasProvider(name)
}

// APIProviderNames returns the list of provider names with API-dispatch support.
// Used by tests, docs sync checks, and resolution boundaries.
func APIProviderNames() []string {
	return extdeploy.APIProviderNames()
}

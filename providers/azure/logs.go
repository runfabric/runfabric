package azure

import (
	"context"
	"fmt"

	"github.com/runfabric/runfabric/internal/config"
	"github.com/runfabric/runfabric/internal/providers"
	"github.com/runfabric/runfabric/internal/state"
)

// Logger returns instructions for Azure Portal log stream.
type Logger struct{}

func (Logger) Logs(ctx context.Context, cfg *config.Config, stage, function string, receipt *state.Receipt) (*providers.LogsResult, error) {
	appName := receipt.Outputs["app_name"]
	if appName == "" {
		appName = fmt.Sprintf("%s-%s", cfg.Service, stage)
	}
	return &providers.LogsResult{
		Provider: "azure-functions",
		Function: function,
		Lines:    []string{"View logs: Azure Portal → Function App " + appName + " → Log stream"},
	}, nil
}

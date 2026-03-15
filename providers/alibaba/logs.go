package alibaba

import (
	"context"

	"github.com/runfabric/runfabric/internal/config"
	"github.com/runfabric/runfabric/internal/providers"
	"github.com/runfabric/runfabric/internal/state"
)

// Logger returns instructions for Alibaba Cloud Console logs.
type Logger struct{}

func (Logger) Logs(ctx context.Context, cfg *config.Config, stage, function string, receipt *state.Receipt) (*providers.LogsResult, error) {
	return &providers.LogsResult{
		Provider: "alibaba-fc",
		Function: function,
		Lines:    []string{"View logs: Alibaba Cloud Console → Function Compute → Logs"},
	}, nil
}

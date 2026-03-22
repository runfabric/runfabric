package alibaba

import (
	"context"
	"fmt"

	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	"github.com/runfabric/runfabric/platform/core/model/config"
	state "github.com/runfabric/runfabric/platform/core/state/core"
)

// Logger returns instructions and console link for Alibaba FC logs.
type Logger struct{}

func (Logger) Logs(ctx context.Context, cfg *config.Config, stage, function string, receipt *state.Receipt) (*providers.LogsResult, error) {
	region := receipt.Outputs["region"]
	if region == "" {
		region = "cn-hangzhou"
	}
	serviceName := receipt.Outputs["service_name"]
	if serviceName == "" {
		serviceName = cfg.Service + "-" + stage
	}
	// Alibaba FC console: region-specific URL for service logs
	consoleLink := fmt.Sprintf("https://fcnext.console.aliyun.com/%s/services/%s (view logs per function)", region, serviceName)
	return &providers.LogsResult{
		Provider: "alibaba-fc",
		Function: function,
		Lines:    []string{"View logs: " + consoleLink},
	}, nil
}

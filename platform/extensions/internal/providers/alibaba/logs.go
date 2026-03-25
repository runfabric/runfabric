package alibaba

import (
	"context"
	"fmt"

	"github.com/runfabric/runfabric/platform/deploy/apiutil"
	"github.com/runfabric/runfabric/platform/extensions/sdkbridge"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// Logger returns instructions and console link for Alibaba FC logs.
type Logger struct{}

func (Logger) Logs(ctx context.Context, cfg sdkprovider.Config, stage, function string, receipt any) (*sdkprovider.LogsResult, error) {
	coreCfg, err := sdkbridge.ToCoreConfig(cfg)
	if err != nil {
		return nil, err
	}
	_ = coreCfg
	rv := apiutil.DecodeReceipt(receipt)
	region := rv.Outputs["region"]
	if region == "" {
		region = "cn-hangzhou"
	}
	serviceName := rv.Outputs["service_name"]
	if serviceName == "" {
		serviceName = coreCfg.Service + "-" + stage
	}
	// Alibaba FC console: region-specific URL for service logs
	consoleLink := fmt.Sprintf("https://fcnext.console.aliyun.com/%s/services/%s (view logs per function)", region, serviceName)
	return &sdkprovider.LogsResult{
		Provider: "alibaba-fc",
		Function: function,
		Lines:    []string{"View logs: " + consoleLink},
	}, nil
}

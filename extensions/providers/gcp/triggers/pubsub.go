package triggers

import (
	"context"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

func EnsurePubSub(ctx context.Context, cfg sdkprovider.Config, stage, functionName, topic string) error {
	_ = cfg
	return nil
}

package triggers

import (
	"context"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

func EnsureQueue(ctx context.Context, cfg sdkprovider.Config, stage, functionName, queue string) error {
	return nil
}

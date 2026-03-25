package triggers

import (
	"context"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

func EnsureStorage(ctx context.Context, cfg sdkprovider.Config, stage, functionName, bucket, prefix string) error {
	return nil
}

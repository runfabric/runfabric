// Package digitalocean re-exports triggers from triggers/ subpackage (per Trigger Capability Matrix).
package digitalocean

import (
	"context"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

func EnsureHTTP(ctx context.Context, cfg sdkprovider.Config, stage, functionName string) error {
	return nil
}

func EnsureCron(ctx context.Context, cfg sdkprovider.Config, stage, functionName, expression string) error {
	return nil
}

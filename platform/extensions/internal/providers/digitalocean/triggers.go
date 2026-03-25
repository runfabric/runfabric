// Package digitalocean re-exports triggers from triggers/ subpackage (per Trigger Capability Matrix).
package digitalocean

import (
	"context"

	"github.com/runfabric/runfabric/platform/extensions/internal/providers/digitalocean/triggers"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// EnsureHTTP delegates to triggers package.
func EnsureHTTP(ctx context.Context, cfg sdkprovider.Config, stage, functionName string) error {
	return triggers.EnsureHTTP(ctx, cfg, stage, functionName)
}

// EnsureCron delegates to triggers package.
func EnsureCron(ctx context.Context, cfg sdkprovider.Config, stage, functionName, expression string) error {
	return triggers.EnsureCron(ctx, cfg, stage, functionName, expression)
}

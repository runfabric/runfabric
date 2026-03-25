// Package resources re-exports trigger hooks from triggers/ subpackage (per Trigger Capability Matrix).
package kubernetes

import (
	"context"

	"github.com/runfabric/runfabric/platform/extensions/internal/providers/kubernetes/triggers"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

func EnsureHTTP(ctx context.Context, cfg sdkprovider.Config, stage, functionName string) error {
	return triggers.EnsureHTTP(ctx, cfg, stage, functionName)
}

func EnsureCron(ctx context.Context, cfg sdkprovider.Config, stage, functionName, expression string) error {
	return triggers.EnsureCron(ctx, cfg, stage, functionName, expression)
}

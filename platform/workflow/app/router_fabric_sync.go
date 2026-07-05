package app

import (
	"io"

	routercontracts "github.com/runfabric/runfabric/platform/core/contracts/router"
)

// RouterSyncFromFabricState syncs the router with the MULTI-CLOUD routing
// config derived from fabric state — the per-provider endpoints recorded by
// RunFabricDeploy (fabric.targets × providerOverrides). This is what puts one
// hostname over every cloud a service is deployed to (failover / latency /
// round-robin per fabric.routing).
//
// Credentials follow the standard chain: per-request daemon headers → router
// policy (apiTokenSecretRef via the secret subsystem, token file) → declared
// same-cloud provider-key fallbacks. Zone/account come from the policy's env
// keys. dryRun true forces a preview regardless of policy.
func RouterSyncFromFabricState(configPath, stage string, dryRun bool, out io.Writer) (*routercontracts.SyncResult, *RouterRoutingConfig, error) {
	ctx, routing, err := routerRoutingFromFabricState(configPath, stage)
	if err != nil {
		return nil, nil, err
	}

	policy := RouterDNSSyncPolicyForStage(ctx.Config, stage)
	zoneID, accountID := RouterProviderIDs(policy)
	if out == nil {
		out = io.Discard
	}
	result, err := RouterDNSSyncWithOptions(ctx, routing, zoneID, accountID, dryRun || policy.DryRun, out, RouterDNSSyncOptions{
		Trigger: "fabric-sync",
	})
	if err != nil {
		return nil, routing, err
	}
	return result, routing, nil
}

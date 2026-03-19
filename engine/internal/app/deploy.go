package app

import (
	"context"
	"os"
	"time"

	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/controlplane"
	deployapi "github.com/runfabric/runfabric/engine/internal/deploy/api"
	awsprovider "github.com/runfabric/runfabric/engine/internal/extensions/provider/aws"
	"github.com/runfabric/runfabric/engine/internal/lifecycle"
	"github.com/runfabric/runfabric/engine/internal/provisioning"
)

// Deploy runs deploy for the given config and stage. If functionName is non-empty, only that function is deployed (when the provider supports it).
// rollbackOnFailure and noRollbackOnFailure are CLI flags; when both false, rollback is resolved from config.Deploy.RollbackOnFailure then RUNFABRIC_ROLLBACK_ON_FAILURE env.
// extraEnv is merged into each function's environment (e.g. for compose service binding SERVICE_*_URL). Can be nil.
func Deploy(configPath, stage, functionName string, rollbackOnFailure, noRollbackOnFailure bool, extraEnv map[string]string, providerOverride string) (any, error) {
	ctx, err := Bootstrap(configPath, stage, providerOverride)
	if err != nil {
		return nil, err
	}

	// Merge managed resource bindings (DATABASE_URL, REDIS_URL, etc.), addon secrets, and extraEnv into each function's environment.
	var provisionFn config.ResourceProvisionFn
	if ctx.Config.Provider.Name != "" {
		p := provisioning.Get(ctx.Config.Provider.Name)
		if p != nil {
			provisionFn = func(provider, key string, spec map[string]any) (string, error) {
				return p.Provision(context.Background(), provider, key, spec)
			}
		}
	}
	resourceEnv, err := config.ResolveResourceBindings(ctx.Config, provisionFn)
	if err != nil {
		return nil, err
	}
	envVarToResource := config.EnvVarToResourceKey(ctx.Config)
	for name, fn := range ctx.Config.Functions {
		if fn.Environment == nil {
			fn.Environment = make(map[string]string)
		}
		// Per-function resource refs: if fn.Resources is set, only inject env from those resource keys; otherwise all.
		for k, v := range resourceEnv {
			if len(fn.Resources) == 0 {
				fn.Environment[k] = v
				continue
			}
			if resourceKey, ok := envVarToResource[k]; ok {
				for _, r := range fn.Resources {
					if r == resourceKey {
						fn.Environment[k] = v
						break
					}
				}
			}
		}
		addonEnv, err := config.ResolveAddonBindingsForKeys(ctx.Config, fn.Addons)
		if err != nil {
			return nil, err
		}
		for k, v := range addonEnv {
			fn.Environment[k] = v
		}
		for k, v := range extraEnv {
			fn.Environment[k] = v
		}
		ctx.Config.Functions[name] = fn
	}

	// Resolve rollback preference: CLI flag > runfabric.yml deploy.rollbackOnFailure > env
	rollback := resolveRollbackOnFailure(ctx, rollbackOnFailure, noRollbackOnFailure)

	provider, err := resolveProvider(ctx)
	if err != nil {
		return nil, err
	}
	var result any

	// Internal providers (currently AWS): full controlplane + adapter.
	if provider.mode == dispatchInternal {
		coord := &controlplane.Coordinator{
			Locks:     ctx.Backends.Locks,
			Journals:  ctx.Backends.Journals,
			Receipts:  ctx.Backends.Receipts,
			LeaseFor:  15 * time.Minute,
			Heartbeat: 30 * time.Second,
		}
		result, err = controlplane.RunDeploy(
			context.Background(),
			coord,
			awsprovider.NewAdapter(),
			ctx.Config,
			ctx.Stage,
			ctx.RootDir,
		)
		if err != nil {
			return nil, err
		}
	} else if provider.mode == dispatchAPI {
		// API-dispatched providers: use deployapi path with stage/root-aware receipt handling.
		result, err = deployapi.Run(context.Background(), provider.name, ctx.Config, ctx.Stage, ctx.RootDir)
		if err != nil {
			return nil, err
		}
	} else {
		// Plugin-dispatched providers: use provider interface path.
		result, err = lifecycle.Deploy(ctx.Registry, ctx.Config, ctx.Stage, ctx.RootDir)
		if err != nil {
			return nil, err
		}
	}

	if err := runPostDeployHealthCheck(ctx.Config, result, configPath, ctx.Stage, providerOverride, rollback); err != nil {
		return nil, err
	}
	return result, nil
}

// resolveRollbackOnFailure returns the effective rollback-on-failure setting. When both CLI flags are false, reads from config then env.
func resolveRollbackOnFailure(ctx *AppContext, rollbackOnFailure, noRollbackOnFailure bool) bool {
	if noRollbackOnFailure {
		return false
	}
	if rollbackOnFailure {
		return true
	}
	if ctx.Config.Deploy != nil && ctx.Config.Deploy.RollbackOnFailure != nil {
		return *ctx.Config.Deploy.RollbackOnFailure
	}
	return os.Getenv("RUNFABRIC_ROLLBACK_ON_FAILURE") == "1" || os.Getenv("RUNFABRIC_ROLLBACK_ON_FAILURE") == "true"
}

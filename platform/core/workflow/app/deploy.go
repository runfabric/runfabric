package app

import (
	"context"
	"os"

	providers "github.com/runfabric/runfabric/internal/provider/contracts"
	"github.com/runfabric/runfabric/platform/core/model/config"
	state "github.com/runfabric/runfabric/platform/core/state/core"
	"github.com/runfabric/runfabric/platform/core/workflow/lifecycle"
	deployapi "github.com/runfabric/runfabric/platform/deploy/core/api"
	"github.com/runfabric/runfabric/platform/deploy/provisioning"
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

	if provider.mode == dispatchAPI {
		// API-dispatched providers: use deployapi path with stage/root-aware receipt handling.
		result, err = deployapi.Run(context.Background(), provider.name, ctx.Config, ctx.Stage, ctx.RootDir)
		if err != nil {
			return nil, err
		}
		if orchestration, ok := provider.provider.(providers.OrchestrationCapable); ok {
			functionResourceByName := map[string]string{}
			if deployRes, ok := result.(*providers.DeployResult); ok {
				for fn, deployed := range deployRes.Functions {
					if deployed.ResourceIdentifier != "" {
						functionResourceByName[fn] = deployed.ResourceIdentifier
					}
				}
			}
			syncRes, err := orchestration.SyncOrchestrations(context.Background(), providers.OrchestrationSyncRequest{
				Config:                 ctx.Config,
				Stage:                  ctx.Stage,
				Root:                   ctx.RootDir,
				FunctionResourceByName: functionResourceByName,
			})
			if err != nil {
				return nil, err
			}
			if deployRes, ok := result.(*providers.DeployResult); ok {
				if deployRes.Metadata == nil {
					deployRes.Metadata = map[string]string{}
				}
				for k, v := range syncRes.Metadata {
					deployRes.Metadata[k] = v
				}
				if deployRes.Outputs == nil {
					deployRes.Outputs = map[string]string{}
				}
				for k, v := range syncRes.Outputs {
					deployRes.Outputs[k] = v
				}
			}
			if receipt, err := state.Load(ctx.RootDir, ctx.Stage); err == nil {
				if receipt.Metadata == nil {
					receipt.Metadata = map[string]string{}
				}
				for k, v := range syncRes.Metadata {
					receipt.Metadata[k] = v
				}
				for k, v := range syncRes.Outputs {
					receipt.Outputs[k] = v
				}
				_ = state.Save(ctx.RootDir, receipt)
			}
		}
	} else {
		// Internal + plugin-dispatched providers: use contract-driven lifecycle path.
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

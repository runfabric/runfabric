package app

import (
	"context"
	"os"

	providers "github.com/runfabric/runfabric/internal/provider/contracts"
	"github.com/runfabric/runfabric/platform/core/model/config"
	state "github.com/runfabric/runfabric/platform/core/state/core"
	deployapi "github.com/runfabric/runfabric/platform/deploy/core/api"
	"github.com/runfabric/runfabric/platform/deploy/provisioning"
	"github.com/runfabric/runfabric/platform/workflow/lifecycle"
	deployusecase "github.com/runfabric/runfabric/platform/workflow/usecase/deploy"
)

// Deploy runs deploy for the given config and stage. If functionName is non-empty, only that function is deployed (when the provider supports it).
// rollbackOnFailure and noRollbackOnFailure are CLI flags; when both false, rollback is resolved from config.Deploy.RollbackOnFailure then RUNFABRIC_ROLLBACK_ON_FAILURE env.
// extraEnv is merged into each function's environment (e.g. for compose service binding SERVICE_*_URL). Can be nil.
func Deploy(configPath, stage, functionName string, rollbackOnFailure, noRollbackOnFailure bool, extraEnv map[string]string, providerOverride string) (any, error) {
	ctx, err := Bootstrap(configPath, stage, providerOverride)
	if err != nil {
		return nil, err
	}

	// Resolve rollback preference: CLI flag > runfabric.yml deploy.rollbackOnFailure > env
	rollback := resolveRollbackOnFailure(ctx, rollbackOnFailure, noRollbackOnFailure)

	result, err := deployusecase.Execute(context.Background(), deployusecase.Input{
		Config:   ctx.Config,
		Registry: ctx.Registry,
		Stage:    ctx.Stage,
		RootDir:  ctx.RootDir,
		ExtraEnv: extraEnv,
	}, deployusecase.Dependencies{
		ResolveProvider: func(cfg *config.Config) (*deployusecase.ProviderResolution, error) {
			resolved, err := resolveProvider(ctx)
			if err != nil {
				return nil, err
			}
			return &deployusecase.ProviderResolution{
				Name:     resolved.name,
				Provider: resolved.provider,
				Mode:     mapDispatchMode(resolved.mode),
			}, nil
		},
		ResolveResourceEnv: func(cfg *config.Config) (map[string]string, error) {
			var provisionFn config.ResourceProvisionFn
			if cfg.Provider.Name != "" {
				p := provisioning.Get(cfg.Provider.Name)
				if p != nil {
					provisionFn = func(provider, key string, spec map[string]any) (string, error) {
						return p.Provision(context.Background(), provider, key, spec)
					}
				}
			}
			return config.ResolveResourceBindings(cfg, provisionFn)
		},
		EnvVarToResourceKey: config.EnvVarToResourceKey,
		ResolveAddonEnvForFn: func(cfg *config.Config, addonKeys []string) (map[string]string, error) {
			return config.ResolveAddonBindingsForKeys(cfg, addonKeys)
		},
		DeployAPI: deployapi.Run,
		DeployLifecycle: func(reg *providers.Registry, cfg *config.Config, stage, root string) (*providers.DeployResult, error) {
			return lifecycle.Deploy(reg, cfg, stage, root)
		},
		MergeOrchestrations: mergeDeployOrchestrations,
	})
	if err != nil {
		return nil, err
	}

	if err := runPostDeployHealthCheck(ctx.Config, result, configPath, ctx.Stage, providerOverride, rollback); err != nil {
		return nil, err
	}
	return result, nil
}

func mapDispatchMode(mode providerDispatchMode) deployusecase.DispatchMode {
	switch mode {
	case dispatchAPI:
		return deployusecase.DispatchAPI
	case dispatchInternal:
		return deployusecase.DispatchInternal
	default:
		return deployusecase.DispatchPlugin
	}
}

func mergeDeployOrchestrations(ctx context.Context, in deployusecase.Input, provider *deployusecase.ProviderResolution, deployRes *providers.DeployResult) error {
	orchestration, ok := provider.Provider.(providers.OrchestrationCapable)
	if !ok {
		return nil
	}
	functionResourceByName := map[string]string{}
	for fn, deployed := range deployRes.Functions {
		if deployed.ResourceIdentifier != "" {
			functionResourceByName[fn] = deployed.ResourceIdentifier
		}
	}
	syncRes, err := orchestration.SyncOrchestrations(ctx, providers.OrchestrationSyncRequest{
		Config:                 in.Config,
		Stage:                  in.Stage,
		Root:                   in.RootDir,
		FunctionResourceByName: functionResourceByName,
	})
	if err != nil {
		return err
	}
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
	if receipt, err := state.Load(in.RootDir, in.Stage); err == nil {
		if receipt.Metadata == nil {
			receipt.Metadata = map[string]string{}
		}
		for k, v := range syncRes.Metadata {
			receipt.Metadata[k] = v
		}
		for k, v := range syncRes.Outputs {
			receipt.Outputs[k] = v
		}
		_ = state.Save(in.RootDir, receipt)
	}
	return nil
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

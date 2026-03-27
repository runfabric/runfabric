package deploy

import (
	"context"
	"fmt"

	providers "github.com/runfabric/runfabric/internal/provider/contracts"
	"github.com/runfabric/runfabric/platform/core/model/config"
)

type DispatchMode int

const (
	DispatchPlugin DispatchMode = iota
	DispatchAPI
	DispatchInternal
)

type ProviderResolution struct {
	Name     string
	Provider providers.ProviderPlugin
	Mode     DispatchMode
}

type Input struct {
	Config   *config.Config
	Registry *providers.Registry
	Stage    string
	RootDir  string
	ExtraEnv map[string]string
}

type Dependencies struct {
	ResolveProvider      func(cfg *config.Config) (*ProviderResolution, error)
	ResolveResourceEnv   func(cfg *config.Config) (map[string]string, error)
	EnvVarToResourceKey  func(cfg *config.Config) map[string]string
	ResolveAddonEnvForFn func(cfg *config.Config, addonKeys []string) (map[string]string, error)
	DeployAPI            func(ctx context.Context, provider string, cfg *config.Config, stage, root string) (*providers.DeployResult, error)
	DeployLifecycle      func(reg *providers.Registry, cfg *config.Config, stage, root string) (*providers.DeployResult, error)
	MergeOrchestrations  func(ctx context.Context, in Input, provider *ProviderResolution, deployRes *providers.DeployResult) error
}

func Execute(ctx context.Context, in Input, deps Dependencies) (any, error) {
	if in.Config == nil {
		return nil, fmt.Errorf("config is required")
	}
	if in.Registry == nil {
		return nil, fmt.Errorf("provider registry is required")
	}
	if deps.ResolveProvider == nil || deps.ResolveResourceEnv == nil || deps.EnvVarToResourceKey == nil || deps.ResolveAddonEnvForFn == nil || deps.DeployAPI == nil || deps.DeployLifecycle == nil {
		return nil, fmt.Errorf("deploy usecase dependencies are incomplete")
	}

	resourceEnv, err := deps.ResolveResourceEnv(in.Config)
	if err != nil {
		return nil, err
	}
	envVarToResource := deps.EnvVarToResourceKey(in.Config)
	for name, fn := range in.Config.Functions {
		merged, mergeErr := mergeFunctionEnvironment(in.Config, fn, resourceEnv, envVarToResource, in.ExtraEnv, deps.ResolveAddonEnvForFn)
		if mergeErr != nil {
			return nil, mergeErr
		}
		in.Config.Functions[name] = merged
	}

	provider, err := deps.ResolveProvider(in.Config)
	if err != nil {
		return nil, err
	}

	if provider.Mode == DispatchAPI {
		deployRes, deployErr := deps.DeployAPI(ctx, provider.Name, in.Config, in.Stage, in.RootDir)
		if deployErr != nil {
			return nil, deployErr
		}
		if deps.MergeOrchestrations != nil {
			if err := deps.MergeOrchestrations(ctx, in, provider, deployRes); err != nil {
				return nil, err
			}
		}
		return deployRes, nil
	}

	return deps.DeployLifecycle(in.Registry, in.Config, in.Stage, in.RootDir)
}

func mergeFunctionEnvironment(
	cfg *config.Config,
	fn config.FunctionConfig,
	resourceEnv map[string]string,
	envVarToResource map[string]string,
	extraEnv map[string]string,
	resolveAddonEnvForFn func(cfg *config.Config, addonKeys []string) (map[string]string, error),
) (config.FunctionConfig, error) {
	if fn.Environment == nil {
		fn.Environment = make(map[string]string)
	}
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
	addonEnv, err := resolveAddonEnvForFn(cfg, fn.Addons)
	if err != nil {
		return config.FunctionConfig{}, err
	}
	for k, v := range addonEnv {
		fn.Environment[k] = v
	}
	for k, v := range extraEnv {
		fn.Environment[k] = v
	}
	return fn, nil
}

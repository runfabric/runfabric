package configapi

import (
	"encoding/json"
	"fmt"

	"github.com/runfabric/runfabric/platform/core/model/config"
	"github.com/runfabric/runfabric/platform/workflow/app"
)

// CoreWorkflowConnector is the daemon->core boundary for config/workflow operations.
// provider selects a providerOverrides key for multi-cloud (same semantics as
// the CLI --provider flag); "" targets the config's default provider.
type CoreWorkflowConnector interface {
	Validate(configPath, stage string) error
	Resolve(configPath, stage string) (*ResolveResponse, error)
	Plan(configPath, stage, provider string) (*PlanResponse, error)
	Deploy(configPath, stage, provider string) (*DeployResponse, error)
	Remove(configPath, stage, provider string) (*RemoveResponse, error)
	Releases(configPath string) (*ReleasesResponse, error)
	ReleaseHistory(configPath, stage string) (*ReleasesResponse, error)
	// FabricDeploy deploys to EVERY fabric.targets provider and returns the
	// fabric state (per-provider endpoints).
	FabricDeploy(configPath, stage string) (*FabricDeployResponse, error)
	// RouterSync puts the router over the fabric endpoints (multi-cloud
	// routing config synced through the configured router plugin).
	RouterSync(configPath, stage string, dryRun bool) (*RouterSyncResponse, error)
}

type coreWorkflowAdapter struct{}

func (coreWorkflowAdapter) Validate(configPath, stage string) error {
	cfg, err := config.Load(configPath)
	if err == nil {
		cfg, err = config.Resolve(cfg, stage)
	}
	if err == nil {
		err = config.Validate(cfg)
	}
	return err
}

func (coreWorkflowAdapter) Resolve(configPath, stage string) (*ResolveResponse, error) {
	cfg, err := config.Load(configPath)
	if err == nil {
		cfg, err = config.Resolve(cfg, stage)
	}
	if err != nil {
		return nil, err
	}
	payload, err := marshalPayload(cfg)
	if err != nil {
		return nil, err
	}
	return &ResolveResponse{Payload: payload}, nil
}

func (coreWorkflowAdapter) Plan(configPath, stage, provider string) (*PlanResponse, error) {
	res, err := app.Plan(configPath, stage, provider)
	if err != nil {
		return nil, err
	}
	payload, err := marshalPayload(res)
	if err != nil {
		return nil, err
	}
	return &PlanResponse{Payload: payload}, nil
}

func (coreWorkflowAdapter) Deploy(configPath, stage, provider string) (*DeployResponse, error) {
	res, err := app.Deploy(configPath, stage, "", false, false, nil, provider)
	if err != nil {
		return nil, err
	}
	payload, err := marshalPayload(res)
	if err != nil {
		return nil, err
	}
	return &DeployResponse{Payload: payload}, nil
}

func (coreWorkflowAdapter) Remove(configPath, stage, provider string) (*RemoveResponse, error) {
	res, err := app.Remove(configPath, stage, provider)
	if err != nil {
		return nil, err
	}
	payload, err := marshalPayload(res)
	if err != nil {
		return nil, err
	}
	return &RemoveResponse{Payload: payload}, nil
}

func (coreWorkflowAdapter) Releases(configPath string) (*ReleasesResponse, error) {
	res, err := app.Releases(configPath)
	if err != nil {
		return nil, err
	}
	payload, err := marshalPayload(res)
	if err != nil {
		return nil, err
	}
	return &ReleasesResponse{Payload: payload}, nil
}

func (coreWorkflowAdapter) ReleaseHistory(configPath, stage string) (*ReleasesResponse, error) {
	res, err := app.ReleaseHistory(configPath, stage)
	if err != nil {
		return nil, err
	}
	payload, err := marshalPayload(res)
	if err != nil {
		return nil, err
	}
	return &ReleasesResponse{Payload: payload}, nil
}

func marshalPayload(v any) (json.RawMessage, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal connector payload: %w", err)
	}
	return json.RawMessage(b), nil
}

func (coreWorkflowAdapter) FabricDeploy(configPath, stage string) (*FabricDeployResponse, error) {
	fabricState, err := app.RunFabricDeploy(configPath, stage, false, false)
	if err != nil {
		return nil, err
	}
	if fabricState == nil {
		return nil, fmt.Errorf("fabric is not configured: set fabric.targets and providerOverrides in the config")
	}
	payload, err := marshalPayload(fabricState)
	if err != nil {
		return nil, err
	}
	return &FabricDeployResponse{Payload: payload}, nil
}

func (coreWorkflowAdapter) RouterSync(configPath, stage string, dryRun bool) (*RouterSyncResponse, error) {
	result, routing, err := app.RouterSyncFromFabricState(configPath, stage, dryRun, nil)
	if err != nil {
		return nil, err
	}
	payload, err := marshalPayload(map[string]any{"routing": routing, "result": result})
	if err != nil {
		return nil, err
	}
	return &RouterSyncResponse{Payload: payload}, nil
}

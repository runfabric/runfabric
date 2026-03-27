package configapi

import (
	"encoding/json"
	"fmt"

	"github.com/runfabric/runfabric/platform/core/model/config"
	"github.com/runfabric/runfabric/platform/workflow/app"
)

// CoreWorkflowConnector is the daemon->core boundary for config/workflow operations.
type CoreWorkflowConnector interface {
	Validate(configPath, stage string) error
	Resolve(configPath, stage string) (*ResolveResponse, error)
	Plan(configPath, stage string) (*PlanResponse, error)
	Deploy(configPath, stage string) (*DeployResponse, error)
	Remove(configPath, stage string) (*RemoveResponse, error)
	Releases(configPath string) (*ReleasesResponse, error)
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

func (coreWorkflowAdapter) Plan(configPath, stage string) (*PlanResponse, error) {
	res, err := app.Plan(configPath, stage, "")
	if err != nil {
		return nil, err
	}
	payload, err := marshalPayload(res)
	if err != nil {
		return nil, err
	}
	return &PlanResponse{Payload: payload}, nil
}

func (coreWorkflowAdapter) Deploy(configPath, stage string) (*DeployResponse, error) {
	res, err := app.Deploy(configPath, stage, "", false, false, nil, "")
	if err != nil {
		return nil, err
	}
	payload, err := marshalPayload(res)
	if err != nil {
		return nil, err
	}
	return &DeployResponse{Payload: payload}, nil
}

func (coreWorkflowAdapter) Remove(configPath, stage string) (*RemoveResponse, error) {
	res, err := app.Remove(configPath, stage, "")
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

func marshalPayload(v any) (json.RawMessage, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal connector payload: %w", err)
	}
	return json.RawMessage(b), nil
}

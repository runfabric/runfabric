package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	builtinrouters "github.com/runfabric/runfabric/extensions/routers"
	providers "github.com/runfabric/runfabric/internal/provider/contracts"
	"github.com/runfabric/runfabric/platform/core/model/config"
	"github.com/runfabric/runfabric/platform/extensions/registry/resolution"
)

// ConnectorProvider is the app-owned provider resolution DTO returned by the connector boundary.
type ConnectorProvider struct {
	Name          string
	Provider      providers.ProviderPlugin
	IsInternal    bool
	IsAPIDispatch bool
}

// ExtensionsConnector is the single app-domain connection interface into platform/extensions.
// It keeps app workflows decoupled from concrete resolver implementations.
type ExtensionsConnector interface {
	ResolveProvider(name string) (*ConnectorProvider, error)
	EnsureSimulator(simulatorID string) error
	BuildFunction(ctx context.Context, req RuntimeBuildRequest) (*providers.Artifact, error)
	Simulate(ctx context.Context, simulatorID string, req SimulatorInvokeRequest) (*SimulatorInvokeResult, error)
	SyncRouter(ctx context.Context, routerID string, req RouterSyncRequest) (*builtinrouters.SyncResult, error)
	RefreshExternal() error
}

// RuntimeBuildRequest captures build inputs at the app-domain boundary.
type RuntimeBuildRequest struct {
	Runtime         string
	Root            string
	FunctionName    string
	FunctionConfig  config.FunctionConfig
	ConfigSignature string
}

// SimulatorInvokeRequest captures local invoke inputs at the app-domain boundary.
type SimulatorInvokeRequest struct {
	Service    string
	Stage      string
	Function   string
	Method     string
	Path       string
	Query      map[string]string
	Headers    map[string]string
	Body       []byte
	WorkDir    string
	HandlerRef string
	Runtime    string
}

// SimulatorInvokeResult is the normalized invoke response returned through the app-domain connector.
type SimulatorInvokeResult struct {
	StatusCode int
	Headers    map[string]string
	Body       json.RawMessage
}

type RouterSyncRequest struct {
	Routing   *RouterRoutingConfig
	ZoneID    string
	AccountID string
	DryRun    bool
	Out       io.Writer
}

type resolutionExtensionsAdapter struct {
	boundary *resolution.Boundary
}

func newExtensionsConnectorFromBoundary(boundary *resolution.Boundary) ExtensionsConnector {
	return &resolutionExtensionsAdapter{boundary: boundary}
}

func (a *resolutionExtensionsAdapter) ResolveProvider(name string) (*ConnectorProvider, error) {
	p, err := a.boundary.ResolveProvider(name)
	if err != nil {
		return nil, err
	}
	return &ConnectorProvider{
		Name:          name,
		Provider:      p,
		IsInternal:    a.boundary.IsInternalProvider(name),
		IsAPIDispatch: a.boundary.IsAPIDispatchProvider(name),
	}, nil
}

func (a *resolutionExtensionsAdapter) EnsureSimulator(simulatorID string) error {
	_, err := a.boundary.ResolveSimulator(simulatorID)
	return err
}

func (a *resolutionExtensionsAdapter) BuildFunction(ctx context.Context, req RuntimeBuildRequest) (*providers.Artifact, error) {
	return a.boundary.BuildFunction(ctx, resolution.RuntimeBuildRequest{
		Runtime:         req.Runtime,
		Root:            req.Root,
		FunctionName:    req.FunctionName,
		FunctionConfig:  req.FunctionConfig,
		ConfigSignature: req.ConfigSignature,
	})
}

func (a *resolutionExtensionsAdapter) Simulate(ctx context.Context, simulatorID string, req SimulatorInvokeRequest) (*SimulatorInvokeResult, error) {
	res, err := a.boundary.Simulate(ctx, simulatorID, resolution.SimulatorInvokeRequest{
		Service:    req.Service,
		Stage:      req.Stage,
		Function:   req.Function,
		Method:     req.Method,
		Path:       req.Path,
		Query:      req.Query,
		Headers:    req.Headers,
		Body:       req.Body,
		WorkDir:    req.WorkDir,
		HandlerRef: req.HandlerRef,
		Runtime:    req.Runtime,
	})
	if err != nil {
		return nil, err
	}
	return &SimulatorInvokeResult{
		StatusCode: res.StatusCode,
		Headers:    res.Headers,
		Body:       res.Body,
	}, nil
}

func (a *resolutionExtensionsAdapter) SyncRouter(ctx context.Context, routerID string, req RouterSyncRequest) (*builtinrouters.SyncResult, error) {
	if req.Routing == nil {
		return nil, fmt.Errorf("routing config is nil")
	}
	endpoints := make([]builtinrouters.RoutingEndpoint, len(req.Routing.Endpoints))
	for i, ep := range req.Routing.Endpoints {
		endpoints[i] = builtinrouters.RoutingEndpoint{
			Name:    ep.Name,
			URL:     ep.URL,
			Healthy: ep.Healthy,
			Weight:  ep.Weight,
		}
	}
	return a.boundary.SyncRouter(ctx, routerID, builtinrouters.SyncRequest{
		Routing: &builtinrouters.RoutingConfig{
			Contract:   req.Routing.Contract,
			Service:    req.Routing.Service,
			Stage:      req.Routing.Stage,
			Hostname:   req.Routing.Hostname,
			Strategy:   req.Routing.Strategy,
			HealthPath: req.Routing.HealthPath,
			TTL:        req.Routing.TTL,
			Endpoints:  endpoints,
		},
		ZoneID:    req.ZoneID,
		AccountID: req.AccountID,
		DryRun:    req.DryRun,
		Out:       req.Out,
	})
}

func (a *resolutionExtensionsAdapter) RefreshExternal() error {
	return a.boundary.RefreshExternal()
}

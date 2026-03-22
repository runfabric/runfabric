package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	"github.com/runfabric/runfabric/platform/core/model/config"
	planner "github.com/runfabric/runfabric/platform/core/planner/engine"
)

func discoverActualState(ctx context.Context, clients *AWSClients, cfg *config.Config, stage string) (*planner.ActualState, error) {
	state := &planner.ActualState{
		Functions:    []planner.ActualFunction{},
		Routes:       []planner.ActualRoute{},
		Integrations: []planner.ActualIntegration{},
	}

	servicePrefix := cfg.Service + "-" + stage + "-"

	pager := lambda.NewListFunctionsPaginator(
		clients.Lambda,
		&lambda.ListFunctionsInput{},
	)

	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list lambdas: %w", err)
		}

		for _, fn := range page.Functions {
			if fn.FunctionName == nil {
				continue
			}
			if !strings.HasPrefix(*fn.FunctionName, servicePrefix) {
				continue
			}

			item := planner.ActualFunction{
				Name: *fn.FunctionName,
			}
			if fn.Runtime != "" {
				item.Runtime = string(fn.Runtime)
			}
			if fn.Handler != nil {
				item.Handler = *fn.Handler
			}
			if fn.FunctionArn != nil {
				item.ResourceIdentifier = *fn.FunctionArn
			}
			if fn.MemorySize != nil {
				item.Memory = *fn.MemorySize
			}
			if fn.Timeout != nil {
				item.Timeout = *fn.Timeout
			}
			if fn.CodeSha256 != nil {
				item.CodeSHA256 = *fn.CodeSha256
			}

			state.Functions = append(state.Functions, item)
		}
	}

	apiName := httpAPIName(cfg, stage)
	apisOut, err := clients.APIGW.GetApis(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("list apis: %w", err)
	}

	for _, api := range apisOut.Items {
		if api.Name != nil && *api.Name == apiName {
			if api.ApiId == nil || api.ApiEndpoint == nil {
				continue
			}

			state.HTTPAPI = &planner.ActualHTTPAPI{
				ID:       *api.ApiId,
				Name:     *api.Name,
				Endpoint: *api.ApiEndpoint,
			}

			routesOut, err := clients.APIGW.GetRoutes(ctx, &apigatewayv2.GetRoutesInput{
				ApiId: api.ApiId,
			})
			if err != nil {
				return nil, fmt.Errorf("list routes: %w", err)
			}

			for _, route := range routesOut.Items {
				item := planner.ActualRoute{}
				if route.RouteId != nil {
					item.ID = *route.RouteId
				}
				if route.RouteKey != nil {
					item.RouteKey = *route.RouteKey
				}
				if route.Target != nil {
					item.Target = *route.Target
				}
				if strings.HasPrefix(item.Target, "integrations/") {
					item.IntegrationID = strings.TrimPrefix(item.Target, "integrations/")
				}
				state.Routes = append(state.Routes, item)
			}

			integrationsOut, err := clients.APIGW.GetIntegrations(ctx, &apigatewayv2.GetIntegrationsInput{
				ApiId: api.ApiId,
			})
			if err != nil {
				return nil, fmt.Errorf("list integrations: %w", err)
			}

			for _, integ := range integrationsOut.Items {
				item := planner.ActualIntegration{}
				if integ.IntegrationId != nil {
					item.ID = *integ.IntegrationId
				}
				if integ.IntegrationUri != nil {
					item.IntegrationURI = *integ.IntegrationUri
				}
				state.Integrations = append(state.Integrations, item)
			}

			break
		}
	}

	return state, nil
}

func desiredStateFromConfig(cfg *config.Config, stage string, artifacts map[string]providers.Artifact) *planner.DesiredState {
	desired := &planner.DesiredState{
		Functions: []planner.DesiredFunction{},
		Routes:    []planner.DesiredRoute{},
	}

	hasHTTP := false

	for fnName, fn := range cfg.Functions {
		artifact := artifacts[fnName]

		desired.Functions = append(desired.Functions, planner.DesiredFunction{
			Name:            functionName(cfg, stage, fnName),
			Runtime:         fn.Runtime,
			Handler:         fn.Handler,
			Memory:          defaultInt(fn.Memory, 128),
			Timeout:         defaultInt(fn.Timeout, 10),
			CodeSHA256:      artifact.SHA256,
			ConfigSignature: artifact.ConfigSignature,
		})

		for _, ev := range fn.Events {
			if ev.HTTP == nil {
				continue
			}
			hasHTTP = true
			desired.Routes = append(desired.Routes, planner.DesiredRoute{
				RouteKey:     strings.ToUpper(ev.HTTP.Method) + " " + ev.HTTP.Path,
				FunctionName: functionName(cfg, stage, fnName),
				Method:       ev.HTTP.Method,
				Path:         ev.HTTP.Path,
			})
		}
	}

	if hasHTTP {
		desired.HTTPAPI = &planner.DesiredHTTPAPI{
			Name: httpAPIName(cfg, stage),
		}
	}

	return desired
}

func actualFunctionMap(actual *planner.ActualState) map[string]planner.ActualFunction {
	out := map[string]planner.ActualFunction{}
	if actual == nil {
		return out
	}
	for _, fn := range actual.Functions {
		out[fn.Name] = fn
	}
	return out
}

func defaultInt(v, def int) int {
	if v > 0 {
		return v
	}
	return def
}

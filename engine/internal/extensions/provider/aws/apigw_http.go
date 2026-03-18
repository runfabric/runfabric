package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/runfabric/runfabric/engine/internal/config"

	apigatewayv2 "github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
)

func ensureHTTPAPI(
	ctx context.Context,
	clients *AWSClients,
	cfg *config.Config,
	stage string,
) (*HTTPAPI, bool, error) {
	name := httpAPIName(cfg, stage)

	apisOut, err := clients.APIGW.GetApis(ctx, &apigatewayv2.GetApisInput{})
	if err != nil {
		return nil, false, fmt.Errorf("list apis: %w", err)
	}

	for _, api := range apisOut.Items {
		if api.Name != nil && *api.Name == name {
			if api.ApiId == nil || api.ApiEndpoint == nil {
				return nil, false, fmt.Errorf("existing api missing id or endpoint")
			}

			stageCfg := stageHTTPConfig(cfg, stage)
			if err := ensureHTTPStage(ctx, clients, *api.ApiId, stage, stageCfg); err != nil {
				return nil, false, err
			}

			return &HTTPAPI{
				APIID:       *api.ApiId,
				APIEndpoint: *api.ApiEndpoint,
				StageName:   resolvedStageName(stageCfg, stage),
			}, false, nil
		}
	}

	createOut, err := clients.APIGW.CreateApi(ctx, &apigatewayv2.CreateApiInput{
		Name:         str(name),
		ProtocolType: apigwtypes.ProtocolTypeHttp,
	})
	if err != nil {
		return nil, false, fmt.Errorf("create api: %w", err)
	}

	if createOut.ApiId == nil || createOut.ApiEndpoint == nil {
		return nil, false, fmt.Errorf("created api missing id or endpoint")
	}

	stageCfg := stageHTTPConfig(cfg, stage)
	if err := ensureHTTPStage(ctx, clients, *createOut.ApiId, stage, stageCfg); err != nil {
		return nil, false, err
	}

	return &HTTPAPI{
		APIID:       *createOut.ApiId,
		APIEndpoint: *createOut.ApiEndpoint,
		StageName:   resolvedStageName(stageCfg, stage),
	}, true, nil
}

func ensureHTTPStage(
	ctx context.Context,
	clients *AWSClients,
	apiID string,
	stage string,
	stageCfg *config.StageHTTPConfig,
) error {
	stageName := resolvedStageName(stageCfg, stage)

	stagesOut, err := clients.APIGW.GetStages(ctx, &apigatewayv2.GetStagesInput{
		ApiId: &apiID,
	})
	if err != nil {
		return fmt.Errorf("list stages: %w", err)
	}

	for _, s := range stagesOut.Items {
		if s.StageName != nil && *s.StageName == stageName {
			if err := ensureHTTPStageSettings(ctx, clients, apiID, stageCfg); err != nil {
				return err
			}
			return nil
		}
	}

	_, err = clients.APIGW.CreateStage(ctx, &apigatewayv2.CreateStageInput{
		ApiId:      &apiID,
		StageName:  &stageName,
		AutoDeploy: boolPtr(true),
	})
	if err != nil {
		return fmt.Errorf("create stage: %w", err)
	}

	if err := ensureHTTPStageSettings(ctx, clients, apiID, stageCfg); err != nil {
		return err
	}

	return nil
}

func ensureHTTPIntegration(
	ctx context.Context,
	clients *AWSClients,
	apiID string,
	lambdaARN string,
) (string, bool, error) {
	integrationsOut, err := clients.APIGW.GetIntegrations(ctx, &apigatewayv2.GetIntegrationsInput{
		ApiId: &apiID,
	})
	if err != nil {
		return "", false, fmt.Errorf("list integrations: %w", err)
	}

	for _, item := range integrationsOut.Items {
		if item.IntegrationUri != nil && *item.IntegrationUri == lambdaARN {
			if item.IntegrationId == nil {
				return "", false, fmt.Errorf("existing integration missing id")
			}
			return *item.IntegrationId, false, nil
		}
	}

	createOut, err := clients.APIGW.CreateIntegration(ctx, &apigatewayv2.CreateIntegrationInput{
		ApiId:                &apiID,
		IntegrationType:      apigwtypes.IntegrationTypeAwsProxy,
		IntegrationUri:       &lambdaARN,
		PayloadFormatVersion: str("2.0"),
	})
	if err != nil {
		return "", false, fmt.Errorf("create integration: %w", err)
	}

	if createOut.IntegrationId == nil {
		return "", false, fmt.Errorf("created integration missing id")
	}

	return *createOut.IntegrationId, true, nil
}

func ensureHTTPRoute(
	ctx context.Context,
	clients *AWSClients,
	apiID string,
	method string,
	path string,
	integrationID string,
	authorizerID string,
	authorizationType string,
) (string, bool, error) {
	// API Gateway HTTP API requires path with leading slash (e.g. GET /hello)
	if path = strings.TrimSpace(path); path != "" && path[0] != '/' {
		path = "/" + path
	}
	if path == "" {
		path = "/"
	}
	routeKey := strings.ToUpper(method) + " " + path

	routesOut, err := clients.APIGW.GetRoutes(ctx, &apigatewayv2.GetRoutesInput{
		ApiId: &apiID,
	})
	if err != nil {
		return "", false, fmt.Errorf("list routes: %w", err)
	}

	target := "integrations/" + integrationID

	for _, route := range routesOut.Items {
		if route.RouteKey != nil && *route.RouteKey == routeKey {
			if route.RouteId == nil {
				return "", false, fmt.Errorf("existing route missing id")
			}

			needsUpdate := false
			updateIn := &apigatewayv2.UpdateRouteInput{
				ApiId:   &apiID,
				RouteId: route.RouteId,
			}

			if route.Target == nil || *route.Target != target {
				updateIn.Target = &target
				needsUpdate = true
			}

			// Authorization diff handling
			currentAuthType := ""
			if route.AuthorizationType != "" {
				currentAuthType = string(route.AuthorizationType)
			}
			if currentAuthType != authorizationType {
				updateIn.AuthorizationType = apigwtypes.AuthorizationType(authorizationType)
				needsUpdate = true
			}

			currentAuthorizerID := ""
			if route.AuthorizerId != nil {
				currentAuthorizerID = *route.AuthorizerId
			}
			if currentAuthorizerID != authorizerID {
				if authorizerID != "" {
					updateIn.AuthorizerId = &authorizerID
				}
				needsUpdate = true
			}

			if needsUpdate {
				if _, err := clients.APIGW.UpdateRoute(ctx, updateIn); err != nil {
					return "", false, fmt.Errorf("update route: %w", err)
				}
			}

			return *route.RouteId, false, nil
		}
	}

	createIn := &apigatewayv2.CreateRouteInput{
		ApiId:    &apiID,
		RouteKey: &routeKey,
		Target:   &target,
	}
	if authorizationType != "" {
		createIn.AuthorizationType = apigwtypes.AuthorizationType(authorizationType)
	}
	if authorizerID != "" {
		createIn.AuthorizerId = &authorizerID
	}

	createOut, err := clients.APIGW.CreateRoute(ctx, createIn)
	if err != nil {
		return "", false, fmt.Errorf("create route: %w", err)
	}

	if createOut.RouteId == nil {
		return "", false, fmt.Errorf("created route missing id")
	}

	return *createOut.RouteId, true, nil
}

func routeInvokeURL(api *HTTPAPI, path string) string {
	base := strings.TrimRight(api.APIEndpoint, "/")
	p := "/" + strings.TrimLeft(path, "/")
	return base + "/" + api.StageName + p
}

func resolvedStageName(stageCfg *config.StageHTTPConfig, fallback string) string {
	if stageCfg != nil && strings.TrimSpace(stageCfg.Name) != "" {
		return stageCfg.Name
	}
	return fallback
}

func stageHTTPConfig(cfg *config.Config, stage string) *config.StageHTTPConfig {
	if cfg == nil || cfg.Stages == nil {
		return nil
	}
	stageCfg, ok := cfg.Stages[stage]
	if !ok || stageCfg.HTTP == nil {
		return nil
	}
	return stageCfg.HTTP
}

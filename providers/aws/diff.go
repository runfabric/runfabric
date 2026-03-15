package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/runfabric/runfabric/internal/config"
	"github.com/runfabric/runfabric/internal/planner"
)

func deleteStaleRoutes(ctx context.Context, clients *AWSClients, apiID string, desired *planner.DesiredState, actual *planner.ActualState) error {
	desiredRouteKeys := map[string]struct{}{}
	for _, r := range desired.Routes {
		desiredRouteKeys[r.RouteKey] = struct{}{}
	}

	for _, route := range actual.Routes {
		if _, ok := desiredRouteKeys[route.RouteKey]; ok {
			continue
		}
		if route.ID == "" {
			continue
		}

		_, err := clients.APIGW.DeleteRoute(ctx, &apigatewayv2.DeleteRouteInput{
			ApiId:   &apiID,
			RouteId: &route.ID,
		})
		if err != nil {
			return fmt.Errorf("delete stale route %s: %w", route.RouteKey, err)
		}
	}

	desiredIntegrationIDs := map[string]struct{}{}
	for _, route := range actual.Routes {
		if _, ok := desiredRouteKeys[route.RouteKey]; ok && route.IntegrationID != "" {
			desiredIntegrationIDs[route.IntegrationID] = struct{}{}
		}
	}

	for _, integ := range actual.Integrations {
		if integ.ID == "" {
			continue
		}
		if _, ok := desiredIntegrationIDs[integ.ID]; ok {
			continue
		}
		_, err := clients.APIGW.DeleteIntegration(ctx, &apigatewayv2.DeleteIntegrationInput{
			ApiId:         &apiID,
			IntegrationId: &integ.ID,
		})
		if err != nil && !strings.Contains(err.Error(), "NotFound") {
			return fmt.Errorf("delete stale integration %s: %w", integ.ID, err)
		}
	}

	return nil
}

func deleteStaleLambdas(ctx context.Context, clients *AWSClients, cfg *config.Config, stage string, desired *planner.DesiredState, actual *planner.ActualState) error {
	desiredLambdaNames := map[string]struct{}{}
	for _, l := range desired.Lambdas {
		desiredLambdaNames[l.Name] = struct{}{}
	}

	for _, l := range actual.Lambdas {
		if _, ok := desiredLambdaNames[l.Name]; ok {
			continue
		}

		_, err := clients.Lambda.DeleteFunction(ctx, &lambda.DeleteFunctionInput{
			FunctionName: &l.Name,
		})
		if err != nil && !isLambdaNotFound(err) {
			return fmt.Errorf("delete stale lambda %s: %w", l.Name, err)
		}
	}

	return nil
}

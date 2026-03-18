package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/runfabric/runfabric/engine/internal/config"

	apigatewayv2 "github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
)

// DevStreamState holds state for redirecting API Gateway to a tunnel and restoring on exit.
type DevStreamState struct {
	APIID               string
	TunnelIntegrationID string
	RouteRestore        []struct{ RouteID, OriginalTarget string }
}

// RedirectToTunnel finds the HTTP API for the service/stage, creates an HTTP_PROXY integration
// to tunnelURL, and points all existing routes at that integration. Call Restore to revert.
// Only applies when the API exists and has routes (API Gateway HTTP API path); no-op for
// Function-URL-only deployments.
func RedirectToTunnel(ctx context.Context, cfg *config.Config, stage, tunnelURL string) (*DevStreamState, error) {
	if cfg == nil || cfg.Provider.Region == "" || tunnelURL == "" {
		return nil, fmt.Errorf("config, region, and tunnel URL required")
	}
	clients, err := loadClients(ctx, cfg.Provider.Region)
	if err != nil {
		return nil, err
	}

	apiName := httpAPIName(cfg, stage)
	apisOut, err := clients.APIGW.GetApis(ctx, &apigatewayv2.GetApisInput{})
	if err != nil {
		return nil, fmt.Errorf("list apis: %w", err)
	}
	var apiID string
	for _, api := range apisOut.Items {
		if api.Name != nil && *api.Name == apiName && api.ApiId != nil {
			apiID = *api.ApiId
			break
		}
	}
	if apiID == "" {
		return nil, fmt.Errorf("api %q not found (deploy first or use manual tunnel)", apiName)
	}

	routesOut, err := clients.APIGW.GetRoutes(ctx, &apigatewayv2.GetRoutesInput{ApiId: &apiID})
	if err != nil {
		return nil, fmt.Errorf("list routes: %w", err)
	}
	if len(routesOut.Items) == 0 {
		return nil, fmt.Errorf("api has no routes to redirect")
	}

	// Normalize tunnel URL (no trailing slash for IntegrationUri)
	tunnelURL = strings.TrimSuffix(tunnelURL, "/")

	createOut, err := clients.APIGW.CreateIntegration(ctx, &apigatewayv2.CreateIntegrationInput{
		ApiId:                &apiID,
		IntegrationType:      apigwtypes.IntegrationTypeHttpProxy,
		IntegrationUri:       &tunnelURL,
		PayloadFormatVersion: str("2.0"),
	})
	if err != nil {
		return nil, fmt.Errorf("create tunnel integration: %w", err)
	}
	if createOut.IntegrationId == nil {
		return nil, fmt.Errorf("created integration missing id")
	}
	tunnelIntegID := *createOut.IntegrationId
	tunnelTarget := "integrations/" + tunnelIntegID

	state := &DevStreamState{
		APIID:               apiID,
		TunnelIntegrationID: tunnelIntegID,
		RouteRestore:        make([]struct{ RouteID, OriginalTarget string }, 0, len(routesOut.Items)),
	}

	for _, route := range routesOut.Items {
		if route.RouteId == nil || route.Target == nil {
			continue
		}
		originalTarget := *route.Target
		// Skip if already pointing at our tunnel (idempotent)
		if originalTarget == tunnelTarget {
			state.RouteRestore = append(state.RouteRestore, struct{ RouteID, OriginalTarget string }{*route.RouteId, originalTarget})
			continue
		}
		_, err = clients.APIGW.UpdateRoute(ctx, &apigatewayv2.UpdateRouteInput{
			ApiId:   &apiID,
			RouteId: route.RouteId,
			Target:  &tunnelTarget,
		})
		if err != nil {
			_ = state.Restore(ctx, cfg.Provider.Region)
			return nil, fmt.Errorf("update route %s: %w", *route.RouteId, err)
		}
		state.RouteRestore = append(state.RouteRestore, struct{ RouteID, OriginalTarget string }{*route.RouteId, originalTarget})
	}

	return state, nil
}

// Restore reverts all routes to their original integration targets and deletes the tunnel integration.
func (s *DevStreamState) Restore(ctx context.Context, region string) error {
	if s == nil || s.APIID == "" {
		return nil
	}
	clients, err := loadClients(ctx, region)
	if err != nil {
		return err
	}
	for _, r := range s.RouteRestore {
		target := r.OriginalTarget
		_, err = clients.APIGW.UpdateRoute(ctx, &apigatewayv2.UpdateRouteInput{
			ApiId:   &s.APIID,
			RouteId: str(r.RouteID),
			Target:  &target,
		})
		if err != nil {
			return fmt.Errorf("restore route %s: %w", r.RouteID, err)
		}
	}
	_, err = clients.APIGW.DeleteIntegration(ctx, &apigatewayv2.DeleteIntegrationInput{
		ApiId:         &s.APIID,
		IntegrationId: &s.TunnelIntegrationID,
	})
	if err != nil {
		return fmt.Errorf("delete tunnel integration: %w", err)
	}
	return nil
}

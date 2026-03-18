package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	"github.com/runfabric/runfabric/engine/internal/config"
)

func buildCORSConfig(ev *config.HTTPEvent) *apigwtypes.Cors {
	if ev == nil || ev.CORS == nil {
		return nil
	}

	c := &apigwtypes.Cors{}

	if len(ev.CORS.AllowOrigins) > 0 {
		c.AllowOrigins = ev.CORS.AllowOrigins
	}
	if len(ev.CORS.AllowMethods) > 0 {
		c.AllowMethods = ev.CORS.AllowMethods
	}
	if len(ev.CORS.AllowHeaders) > 0 {
		c.AllowHeaders = ev.CORS.AllowHeaders
	}
	if len(ev.CORS.ExposeHeaders) > 0 {
		c.ExposeHeaders = ev.CORS.ExposeHeaders
	}
	c.AllowCredentials = aws.Bool(ev.CORS.AllowCredentials)
	if ev.CORS.MaxAge > 0 {
		v := int32(ev.CORS.MaxAge)
		c.MaxAge = &v
	}

	return c
}

func ensureHTTPCORS(ctx context.Context, clients *AWSClients, apiID string, ev *config.HTTPEvent) error {
	cors := buildCORSConfig(ev)
	if cors == nil {
		return nil
	}

	_, err := clients.APIGW.UpdateApi(ctx, &apigatewayv2.UpdateApiInput{
		ApiId:             &apiID,
		CorsConfiguration: cors,
	})
	return err
}

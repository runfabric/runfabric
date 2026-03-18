package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	"github.com/runfabric/runfabric/engine/internal/config"
)

func ensureHTTPStageSettings(ctx context.Context, clients *AWSClients, apiID string, stageCfg *config.StageHTTPConfig) error {
	if stageCfg == nil {
		return nil
	}

	stageName := stageCfg.Name
	if stageName == "" {
		stageName = "$default"
	}

	// access logging and tags can be expanded later
	_, err := clients.APIGW.UpdateStage(ctx, &apigatewayv2.UpdateStageInput{
		ApiId:     &apiID,
		StageName: &stageName,
	})
	return err
}

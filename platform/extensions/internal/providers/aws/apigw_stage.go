package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	cloudwatchlogs "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/runfabric/runfabric/platform/core/model/config"
)

func ensureHTTPStageSettings(ctx context.Context, clients *AWSClients, apiID string, stageCfg *config.StageHTTPConfig) error {
	if stageCfg == nil {
		return nil
	}

	stageName := stageCfg.Name
	if stageName == "" {
		stageName = "$default"
	}

	needsAccessLogging := stageCfg.AccessLogging
	needsTags := len(stageCfg.Tags) > 0
	if !needsAccessLogging && !needsTags {
		return nil
	}

	updateInput := buildHTTPStageUpdateInput(apiID, stageName)
	if needsAccessLogging {
		destARN, err := ensureStageAccessLogDestination(ctx, clients, apiID, stageName)
		if err != nil {
			return err
		}
		updateInput.AccessLogSettings = &apigwtypes.AccessLogSettings{
			DestinationArn: str(destARN),
			Format:         str(defaultStageAccessLogFormat),
		}
	}
	if _, err := clients.APIGW.UpdateStage(ctx, updateInput); err != nil {
		return fmt.Errorf("update stage settings: %w", err)
	}

	if needsTags {
		stageARN := stageResourceARN(clients.AWS.Region, apiID, stageName)
		if _, err := clients.APIGW.TagResource(ctx, &apigatewayv2.TagResourceInput{
			ResourceArn: str(stageARN),
			Tags:        stageCfg.Tags,
		}); err != nil {
			return fmt.Errorf("tag stage: %w", err)
		}
	}
	return nil
}

func buildHTTPStageUpdateInput(apiID, stageName string) *apigatewayv2.UpdateStageInput {
	return &apigatewayv2.UpdateStageInput{
		ApiId:     &apiID,
		StageName: &stageName,
	}
}

func ensureStageAccessLogDestination(ctx context.Context, clients *AWSClients, apiID, stageName string) (string, error) {
	logGroupName := stageAccessLogGroupName(apiID, stageName)
	_, err := clients.Logs.CreateLogGroup(ctx, &cloudwatchlogs.CreateLogGroupInput{
		LogGroupName: str(logGroupName),
	})
	if err != nil && !isLogsAlreadyExists(err) {
		return "", fmt.Errorf("create stage access log group: %w", err)
	}
	if strings.TrimSpace(clients.AccountID) == "" {
		return "", fmt.Errorf("cannot configure API Gateway access logging without resolved AWS account ID")
	}
	return stageAccessLogGroupARN(clients.AWS.Region, clients.AccountID, logGroupName), nil
}

func stageResourceARN(region, apiID, stageName string) string {
	return fmt.Sprintf("arn:aws:apigateway:%s::/apis/%s/stages/%s", strings.TrimSpace(region), apiID, stageName)
}

func stageAccessLogGroupName(apiID, stageName string) string {
	return fmt.Sprintf("/aws/apigateway/runfabric/%s/%s", apiID, stageName)
}

func stageAccessLogGroupARN(region, accountID, logGroupName string) string {
	return fmt.Sprintf("arn:aws:logs:%s:%s:log-group:%s", strings.TrimSpace(region), strings.TrimSpace(accountID), logGroupName)
}

const defaultStageAccessLogFormat = `{"requestId":"$context.requestId","ip":"$context.identity.sourceIp","requestTime":"$context.requestTime","httpMethod":"$context.httpMethod","routeKey":"$context.routeKey","status":"$context.status","protocol":"$context.protocol","responseLength":"$context.responseLength"}`

package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/runfabric/runfabric/platform/core/model/config"
)

func ensureAPIGatewayInvokePermission(
	ctx context.Context,
	clients *AWSClients,
	cfg *config.Config,
	stage string,
	stageName string, // actual API Gateway stage name (e.g. "dev") for SourceArn
	fn string,
	method string,
	path string,
	apiID string,
	functionName string,
) error {
	statementID := lambdaPermissionStatementID(cfg, stage, fn, method, path)
	// HTTP API permission format: arn:aws:execute-api:region:account:api-id/stage/*/path (path without leading slash)
	pathPart := strings.TrimPrefix(strings.TrimSpace(path), "/")
	sourceArn := fmt.Sprintf(
		"arn:aws:execute-api:%s:%s:%s/%s/*/%s",
		clients.AWS.Region,
		clients.AccountID,
		apiID,
		stageName,
		pathPart,
	)

	// Remove existing permission with same statement ID so we can re-add with correct SourceArn (e.g. after fixing stage/path format)
	_, _ = clients.Lambda.RemovePermission(ctx, &lambda.RemovePermissionInput{
		FunctionName: &functionName,
		StatementId:  &statementID,
	})
	_, err := clients.Lambda.AddPermission(ctx, &lambda.AddPermissionInput{
		FunctionName: &functionName,
		StatementId:  &statementID,
		Action:       str("lambda:InvokeFunction"),
		Principal:    str("apigateway.amazonaws.com"),
		SourceArn:    &sourceArn,
	})
	if err != nil {
		return fmt.Errorf("add api gateway invoke permission: %w", err)
	}

	return nil
}

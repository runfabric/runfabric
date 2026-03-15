package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/runfabric/runfabric/internal/config"
)

func ensureAPIGatewayInvokePermission(
	ctx context.Context,
	clients *AWSClients,
	cfg *config.Config,
	stage string,
	fn string,
	method string,
	path string,
	apiID string,
	functionName string,
) error {
	statementID := lambdaPermissionStatementID(cfg, stage, fn, method, path)
	sourceArn := fmt.Sprintf(
		"arn:aws:execute-api:%s:%s:%s/*/%s%s",
		clients.AWS.Region,
		clients.AccountID,
		apiID,
		method,
		path,
	)

	_, err := clients.Lambda.AddPermission(ctx, &lambda.AddPermissionInput{
		FunctionName: &functionName,
		StatementId:  &statementID,
		Action:       str("lambda:InvokeFunction"),
		Principal:    str("apigateway.amazonaws.com"),
		SourceArn:    &sourceArn,
	})
	if err != nil && !isLambdaConflict(err) {
		return fmt.Errorf("add api gateway invoke permission: %w", err)
	}

	return nil
}

package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/lambda"
)

func ensureFunctionURL(ctx context.Context, clients *AWSClients, functionName string) (string, bool, error) {
	getOut, err := clients.Lambda.GetFunctionUrlConfig(ctx, &lambda.GetFunctionUrlConfigInput{
		FunctionName: &functionName,
	})

	if err == nil && getOut != nil && getOut.FunctionUrl != nil {
		return *getOut.FunctionUrl, false, nil
	}

	if err != nil && !isLambdaNotFound(err) {
		return "", false, fmt.Errorf("get function url: %w", err)
	}

	createOut, err := clients.Lambda.CreateFunctionUrlConfig(ctx, &lambda.CreateFunctionUrlConfigInput{
		FunctionName: &functionName,
		AuthType:     "NONE",
		InvokeMode:   "BUFFERED",
	})
	if err != nil {
		return "", false, fmt.Errorf("create function url: %w", err)
	}

	_, err = clients.Lambda.AddPermission(ctx, &lambda.AddPermissionInput{
		FunctionName:        &functionName,
		StatementId:         str("FunctionURLAllowPublicAccess"),
		Action:              str("lambda:InvokeFunctionUrl"),
		Principal:           str("*"),
		FunctionUrlAuthType: "NONE",
	})
	if err != nil && !isLambdaConflict(err) {
		return "", false, fmt.Errorf("add function url permission: %w", err)
	}

	if createOut.FunctionUrl == nil {
		return "", false, fmt.Errorf("function url missing from response")
	}

	return *createOut.FunctionUrl, true, nil
}

func deleteFunctionURL(ctx context.Context, clients *AWSClients, functionName string) error {
	_, err := clients.Lambda.DeleteFunctionUrlConfig(ctx, &lambda.DeleteFunctionUrlConfigInput{
		FunctionName: &functionName,
	})
	if err != nil && !isLambdaNotFound(err) {
		return fmt.Errorf("delete function url: %w", err)
	}
	return nil
}

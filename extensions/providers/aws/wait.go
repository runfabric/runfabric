package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

func waitUntilFunctionActive(ctx context.Context, clients *AWSClients, functionName string) error {
	return retry(ctx, 20, 3*time.Second, func() error {
		out, err := clients.Lambda.GetFunction(ctx, &lambda.GetFunctionInput{
			FunctionName: &functionName,
		})
		if err != nil {
			return err
		}

		if out.Configuration == nil || out.Configuration.State == "" {
			return fmt.Errorf("lambda state unavailable")
		}

		if string(out.Configuration.State) != "Active" {
			return fmt.Errorf("lambda not active yet: %s", out.Configuration.State)
		}

		return nil
	})
}

func waitUntilFunctionReady(ctx context.Context, clients *AWSClients, functionName string) error {
	return retry(ctx, 30, 3*time.Second, func() error {
		out, err := clients.Lambda.GetFunction(ctx, &lambda.GetFunctionInput{
			FunctionName: &functionName,
		})
		if err != nil {
			return err
		}

		if out.Configuration == nil {
			return fmt.Errorf("lambda configuration unavailable")
		}

		if out.Configuration.State == "" || out.Configuration.State != lambdatypes.StateActive {
			return fmt.Errorf("lambda not active yet: %s", out.Configuration.State)
		}

		switch out.Configuration.LastUpdateStatus {
		case "", lambdatypes.LastUpdateStatusSuccessful:
			return nil
		case lambdatypes.LastUpdateStatusFailed:
			return fmt.Errorf("lambda update failed: %s", awssdkString(out.Configuration.LastUpdateStatusReason))
		default:
			return fmt.Errorf("lambda update still in progress: %s", out.Configuration.LastUpdateStatus)
		}
	})
}

func awssdkString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func waitUntilFunctionDeleted(ctx context.Context, clients *AWSClients, functionName string) error {
	return retry(ctx, 20, 3*time.Second, func() error {
		_, err := clients.Lambda.GetFunction(ctx, &lambda.GetFunctionInput{
			FunctionName: &functionName,
		})

		if err != nil {
			if isLambdaNotFound(err) {
				return nil
			}
			return err
		}
		return fmt.Errorf("lambda %s still exists", functionName)
	})
}

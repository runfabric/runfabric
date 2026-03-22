package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/lambda"
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

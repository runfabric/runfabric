package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/lambda"
)

func ensureLambdaTags(ctx context.Context, clients *AWSClients, functionArn string, tags map[string]string) error {
	if len(tags) == 0 {
		return nil
	}
	_, err := clients.Lambda.TagResource(ctx, &lambda.TagResourceInput{
		Resource: &functionArn,
		Tags:     tags,
	})
	return err
}

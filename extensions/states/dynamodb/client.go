package dynamodb

import (
	"context"

	dynamodbv2 "github.com/aws/aws-sdk-go-v2/service/dynamodb"

	"github.com/runfabric/runfabric/extensions/states/awsauth"
)

type Client struct {
	DB        *dynamodbv2.Client
	TableName string
}

func New(ctx context.Context, region, tableName string) (*Client, error) {
	// Scoped RUNFABRIC_STATE_AWS_* identity wins over the default chain, so
	// state can live in a different account than the deploy target.
	cfg, err := awsauth.LoadConfig(ctx, region)
	if err != nil {
		return nil, err
	}

	return &Client{
		DB:        dynamodbv2.NewFromConfig(cfg),
		TableName: tableName,
	}, nil
}

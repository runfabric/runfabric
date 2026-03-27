package dynamodb

import (
	"context"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	dynamodbv2 "github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

type Client struct {
	DB        *dynamodbv2.Client
	TableName string
}

func New(ctx context.Context, region, tableName string) (*Client, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, err
	}

	return &Client{
		DB:        dynamodbv2.NewFromConfig(cfg),
		TableName: tableName,
	}, nil
}

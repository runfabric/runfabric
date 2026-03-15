package s3

import (
	"context"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	s3v2 "github.com/aws/aws-sdk-go-v2/service/s3"
)

type Client struct {
	S3     *s3v2.Client
	Bucket string
	Prefix string
}

func New(ctx context.Context, region, bucket, prefix string) (*Client, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, err
	}

	return &Client{
		S3:     s3v2.NewFromConfig(cfg),
		Bucket: bucket,
		Prefix: prefix,
	}, nil
}

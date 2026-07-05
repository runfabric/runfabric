package s3

import (
	"context"

	s3v2 "github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/runfabric/runfabric/extensions/states/awsauth"
)

type Client struct {
	S3     *s3v2.Client
	Bucket string
	Prefix string
}

func New(ctx context.Context, region, bucket, prefix string) (*Client, error) {
	// Scoped RUNFABRIC_STATE_AWS_* identity wins over the default chain, so
	// state can live in a different account than the deploy target.
	cfg, err := awsauth.LoadConfig(ctx, region)
	if err != nil {
		return nil, err
	}

	return &Client{
		S3:     s3v2.NewFromConfig(cfg),
		Bucket: bucket,
		Prefix: prefix,
	}, nil
}

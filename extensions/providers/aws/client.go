package aws

import (
	"context"

	awsv2 "github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	apigwv2 "github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	cloudwatchv2 "github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cloudwatchlogsv2 "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	eventbridgev2 "github.com/aws/aws-sdk-go-v2/service/eventbridge"
	iamv2 "github.com/aws/aws-sdk-go-v2/service/iam"
	lambdav2 "github.com/aws/aws-sdk-go-v2/service/lambda"
	s3v2 "github.com/aws/aws-sdk-go-v2/service/s3"
	sfnv2 "github.com/aws/aws-sdk-go-v2/service/sfn"
	sqsv2 "github.com/aws/aws-sdk-go-v2/service/sqs"
	stsv2 "github.com/aws/aws-sdk-go-v2/service/sts"
	xrayv2 "github.com/aws/aws-sdk-go-v2/service/xray"
)

type AWSClients struct {
	AWS         awsv2.Config
	IAM         *iamv2.Client
	Lambda      *lambdav2.Client
	Logs        *cloudwatchlogsv2.Client
	CloudWatch  *cloudwatchv2.Client
	APIGW       *apigwv2.Client
	EventBridge *eventbridgev2.Client
	S3          *s3v2.Client
	SFN         *sfnv2.Client
	SQS         *sqsv2.Client
	STS         *stsv2.Client
	XRay        *xrayv2.Client
	AccountID   string
}

func loadClients(ctx context.Context, region string) (*AWSClients, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, err
	}

	stsClient := stsv2.NewFromConfig(cfg)
	ident, err := stsClient.GetCallerIdentity(ctx, nil)
	if err != nil {
		return nil, err
	}

	accountID := ""
	if ident.Account != nil {
		accountID = *ident.Account
	}

	return &AWSClients{
		AWS:         cfg,
		IAM:         iamv2.NewFromConfig(cfg),
		Lambda:      lambdav2.NewFromConfig(cfg),
		Logs:        cloudwatchlogsv2.NewFromConfig(cfg),
		CloudWatch:  cloudwatchv2.NewFromConfig(cfg),
		APIGW:       apigwv2.NewFromConfig(cfg),
		EventBridge: eventbridgev2.NewFromConfig(cfg),
		S3:          s3v2.NewFromConfig(cfg),
		SFN:         sfnv2.NewFromConfig(cfg),
		SQS:         sqsv2.NewFromConfig(cfg),
		STS:         stsClient,
		XRay:        xrayv2.NewFromConfig(cfg),
		AccountID:   accountID,
	}, nil
}

// Package awsauth resolves the AWS identity for state backends (s3, dynamodb).
//
// When the scoped RUNFABRIC_STATE_AWS_* variables are set they take precedence
// over the AWS default chain, so state can live in a different AWS account
// than the deploy target (X-Provider-Aws-* / AWS_*) and the aws-sm secret
// source (RUNFABRIC_SM_AWS_*). Unset, behavior is unchanged: the default
// chain (env, shared config/profile, IMDS, IRSA) applies.
package awsauth

import (
	"context"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

// Scoped env keys the state backends honor ahead of the AWS default chain.
const (
	EnvAccessKeyID     = "RUNFABRIC_STATE_AWS_ACCESS_KEY_ID"
	EnvSecretAccessKey = "RUNFABRIC_STATE_AWS_SECRET_ACCESS_KEY"
	EnvSessionToken    = "RUNFABRIC_STATE_AWS_SESSION_TOKEN"
	EnvRegion          = "RUNFABRIC_STATE_AWS_REGION"
)

// LoadConfig builds the AWS config for a state backend: scoped static
// credentials and region override when set, the default chain otherwise.
func LoadConfig(ctx context.Context, region string) (aws.Config, error) {
	return loadConfig(ctx, region, os.Getenv)
}

// loadConfig is the testable core; getenv is injected by tests.
func loadConfig(ctx context.Context, region string, getenv func(string) string) (aws.Config, error) {
	if scoped := strings.TrimSpace(getenv(EnvRegion)); scoped != "" {
		region = scoped
	}
	opts := []func(*awsconfig.LoadOptions) error{}
	if strings.TrimSpace(region) != "" {
		opts = append(opts, awsconfig.WithRegion(region))
	}
	access := strings.TrimSpace(getenv(EnvAccessKeyID))
	secret := strings.TrimSpace(getenv(EnvSecretAccessKey))
	if access != "" && secret != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(access, secret, strings.TrimSpace(getenv(EnvSessionToken)))))
	}
	return awsconfig.LoadDefaultConfig(ctx, opts...)
}

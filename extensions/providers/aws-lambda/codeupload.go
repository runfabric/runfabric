package aws

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	s3v2 "github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// Lambda's direct-upload request cap is ~50MB after base64 inflation; stay under it.
const maxInlineZipBytes = 45 << 20

// codeRef is where the function code lives: inline zip bytes, or an S3 object
// for bundles too large for the Lambda API's inline path.
type codeRef struct {
	zipBytes []byte
	s3Bucket string
	s3Key    string
}

func (c codeRef) inline() bool { return c.s3Bucket == "" }

// codeBucketAPI is the S3 slice used by the code-upload path; *s3.Client satisfies it.
type codeBucketAPI interface {
	HeadBucket(ctx context.Context, in *s3v2.HeadBucketInput, opts ...func(*s3v2.Options)) (*s3v2.HeadBucketOutput, error)
	CreateBucket(ctx context.Context, in *s3v2.CreateBucketInput, opts ...func(*s3v2.Options)) (*s3v2.CreateBucketOutput, error)
	PutObject(ctx context.Context, in *s3v2.PutObjectInput, opts ...func(*s3v2.Options)) (*s3v2.PutObjectOutput, error)
}

func defaultCodeBucket(accountID, region string) string {
	return fmt.Sprintf("runfabric-code-%s-%s", accountID, region)
}

// codeObjectKey is content-addressed so an unchanged re-deploy uploads idempotently.
func codeObjectKey(service, stage string, zipBytes []byte) string {
	sum := sha256.Sum256(zipBytes)
	return fmt.Sprintf("code/%s/%s/%x.zip", service, stage, sum)
}

// resolveCodeRef decides inline vs S3 for the shared deploy zip and uploads when
// needed. The deploying credentials need s3:HeadBucket/CreateBucket/PutObject for
// the S3 path, and an override bucket (RUNFABRIC_AWS_CODE_BUCKET) must live in the
// function's region — Lambda requires same-region code buckets (the default name
// embeds the region for exactly that reason). Content-addressed keys accumulate
// over time; attach a lifecycle rule to the bucket if that matters.
func resolveCodeRef(ctx context.Context, clients *AWSClients, service, stage, region string, zipBytes []byte) (codeRef, error) {
	if int64(len(zipBytes)) <= maxInlineZipBytes {
		return codeRef{zipBytes: zipBytes}, nil
	}
	bucket := sdkprovider.Env("RUNFABRIC_AWS_CODE_BUCKET")
	if bucket == "" {
		bucket = defaultCodeBucket(clients.AccountID, region)
	}
	key := codeObjectKey(service, stage, zipBytes)
	if err := ensureCodeBucket(ctx, clients.S3, bucket, region); err != nil {
		return codeRef{}, fmt.Errorf("ensure code bucket %s: %w", bucket, err)
	}
	if _, err := clients.S3.PutObject(ctx, &s3v2.PutObjectInput{
		Bucket: awssdk.String(bucket),
		Key:    awssdk.String(key),
		Body:   bytes.NewReader(zipBytes),
	}); err != nil {
		return codeRef{}, fmt.Errorf("upload code to s3://%s/%s: %w", bucket, key, err)
	}
	return codeRef{s3Bucket: bucket, s3Key: key}, nil
}

func ensureCodeBucket(ctx context.Context, api codeBucketAPI, bucket, region string) error {
	if _, err := api.HeadBucket(ctx, &s3v2.HeadBucketInput{Bucket: awssdk.String(bucket)}); err == nil {
		return nil
	}
	in := &s3v2.CreateBucketInput{Bucket: awssdk.String(bucket)}
	// us-east-1 must NOT set a LocationConstraint.
	if region != "us-east-1" {
		in.CreateBucketConfiguration = &s3types.CreateBucketConfiguration{
			LocationConstraint: s3types.BucketLocationConstraint(region),
		}
	}
	if _, err := api.CreateBucket(ctx, in); err != nil {
		var owned *s3types.BucketAlreadyOwnedByYou
		if errors.As(err, &owned) {
			return nil
		}
		return err
	}
	// The bucket must be visible before PutObject; the waiter accepts our interface.
	return s3v2.NewBucketExistsWaiter(api).Wait(ctx, &s3v2.HeadBucketInput{Bucket: awssdk.String(bucket)}, 60*time.Second)
}

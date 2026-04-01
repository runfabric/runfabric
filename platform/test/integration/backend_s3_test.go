package integration

import (
	"context"
	"os"
	"testing"

	"github.com/runfabric/runfabric/platform/state/backends"
)

func TestS3BackendIfEnabled(t *testing.T) {
	if os.Getenv("RUNFABRIC_S3_BACKEND_TEST") != "1" {
		t.Skip("set RUNFABRIC_S3_BACKEND_TEST=1 to enable S3 backend test")
	}

	bucket := os.Getenv("RUNFABRIC_S3_BUCKET")
	region := os.Getenv("AWS_REGION")
	if bucket == "" || region == "" {
		t.Fatal("missing RUNFABRIC_S3_BUCKET or AWS_REGION")
	}

	bundle, err := backends.NewBundle(context.Background(), backends.Options{
		Kind:      "s3",
		AWSRegion: region,
		S3Bucket:  bucket,
		S3Prefix:  "runfabric-test",
	})
	if err != nil {
		t.Fatal(err)
	}

	if bundle == nil || bundle.Receipts == nil {
		t.Fatal("expected s3 backend bundle")
	}
	if got := bundle.Receipts.Kind(); got != "s3" {
		t.Fatalf("receipt backend kind=%q want s3", got)
	}
}

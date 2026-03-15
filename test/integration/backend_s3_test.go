package integration

import (
	"context"
	"os"
	"testing"

	s3backend "github.com/runfabric/runfabric/internal/backends/s3"
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

	client, err := s3backend.New(context.Background(), region, bucket, "runfabric-test")
	if err != nil {
		t.Fatal(err)
	}

	if client == nil || client.S3 == nil {
		t.Fatal("expected s3 client")
	}
}

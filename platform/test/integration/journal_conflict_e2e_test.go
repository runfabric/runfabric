package integration

import (
	"context"
	"os"
	"testing"

	s3backend "github.com/runfabric/runfabric/platform/core/state/backends/s3"
	"github.com/runfabric/runfabric/platform/core/state/transactions"
)

func TestJournalConflictE2EWithS3IfEnabled(t *testing.T) {
	if os.Getenv("RUNFABRIC_S3_BACKEND_TEST") != "1" {
		t.Skip("set RUNFABRIC_S3_BACKEND_TEST=1 to enable S3 conflict integration test")
	}

	bucket := os.Getenv("RUNFABRIC_S3_BUCKET")
	region := os.Getenv("AWS_REGION")
	prefix := os.Getenv("RUNFABRIC_S3_PREFIX")
	if bucket == "" || region == "" {
		t.Fatal("missing RUNFABRIC_S3_BUCKET or AWS_REGION")
	}
	if prefix == "" {
		prefix = "runfabric-test"
	}

	client, err := s3backend.New(context.Background(), region, bucket, prefix)
	if err != nil {
		t.Fatalf("init s3 backend failed: %v", err)
	}

	backend := s3backend.NewJournalBackend(context.Background(), client)

	j := &transactions.JournalFile{
		Service:   "conflict-svc",
		Stage:     "dev",
		Operation: "deploy",
		Version:   10,
	}
	sum, err := transactions.ComputeChecksum(j)
	if err != nil {
		t.Fatalf("checksum failed: %v", err)
	}
	j.Checksum = sum

	if err := backend.Save(j); err != nil {
		t.Fatalf("initial backend save failed: %v", err)
	}

	stale := *j
	stale.Version = 9
	sum2, err := transactions.ComputeChecksum(&stale)
	if err != nil {
		t.Fatalf("checksum stale failed: %v", err)
	}
	stale.Checksum = sum2

	err = backend.Save(&stale)
	if err == nil {
		t.Fatal("expected stale S3 journal save to fail")
	}
}

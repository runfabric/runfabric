package integration

import (
	"context"
	"os"
	"testing"

	statetypes "github.com/runfabric/runfabric/internal/state/types"
	"github.com/runfabric/runfabric/platform/state/backends"
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

	bundle, err := backends.NewBundle(context.Background(), backends.Options{
		Kind:      "s3",
		AWSRegion: region,
		S3Bucket:  bucket,
		S3Prefix:  prefix,
	})
	if err != nil {
		t.Fatalf("init s3 backend failed: %v", err)
	}
	if bundle == nil || bundle.Journals == nil {
		t.Fatal("expected s3 journal backend bundle")
	}

	backend := bundle.Journals

	j := &statetypes.JournalFile{
		Service:   "conflict-svc",
		Stage:     "dev",
		Operation: "deploy",
		Version:   10,
	}

	if err := backend.Save(j); err != nil {
		t.Fatalf("initial backend save failed: %v", err)
	}

	stale := *j
	stale.Version = 9

	err = backend.Save(&stale)
	if err == nil {
		t.Fatal("expected stale S3 journal save to fail")
	}
}

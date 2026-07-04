package aws

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	s3v2 "github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type fakeCodeBucketAPI struct {
	headErrs    []error // consumed per HeadBucket call; nil past the end
	headCalls   int
	createCalls []*s3v2.CreateBucketInput
	createErr   error
	putCalls    int
}

func (f *fakeCodeBucketAPI) HeadBucket(_ context.Context, _ *s3v2.HeadBucketInput, _ ...func(*s3v2.Options)) (*s3v2.HeadBucketOutput, error) {
	f.headCalls++
	if f.headCalls <= len(f.headErrs) && f.headErrs[f.headCalls-1] != nil {
		return nil, f.headErrs[f.headCalls-1]
	}
	return &s3v2.HeadBucketOutput{}, nil
}

func (f *fakeCodeBucketAPI) CreateBucket(_ context.Context, in *s3v2.CreateBucketInput, _ ...func(*s3v2.Options)) (*s3v2.CreateBucketOutput, error) {
	f.createCalls = append(f.createCalls, in)
	if f.createErr != nil {
		return nil, f.createErr
	}
	return &s3v2.CreateBucketOutput{}, nil
}

func (f *fakeCodeBucketAPI) PutObject(_ context.Context, _ *s3v2.PutObjectInput, _ ...func(*s3v2.Options)) (*s3v2.PutObjectOutput, error) {
	f.putCalls++
	return &s3v2.PutObjectOutput{}, nil
}

func TestEnsureCodeBucketExisting(t *testing.T) {
	f := &fakeCodeBucketAPI{}
	if err := ensureCodeBucket(context.Background(), f, "b", "ap-south-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.createCalls) != 0 {
		t.Fatalf("expected no CreateBucket for an existing bucket, got %d", len(f.createCalls))
	}
}

func TestEnsureCodeBucketCreatesWithRegionConstraint(t *testing.T) {
	f := &fakeCodeBucketAPI{headErrs: []error{errors.New("404")}}
	if err := ensureCodeBucket(context.Background(), f, "b", "ap-south-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.createCalls) != 1 {
		t.Fatalf("expected exactly one CreateBucket, got %d", len(f.createCalls))
	}
	cfgIn := f.createCalls[0].CreateBucketConfiguration
	if cfgIn == nil || cfgIn.LocationConstraint != s3types.BucketLocationConstraint("ap-south-1") {
		t.Fatalf("expected LocationConstraint ap-south-1, got %+v", cfgIn)
	}
}

func TestEnsureCodeBucketUsEast1OmitsConstraint(t *testing.T) {
	f := &fakeCodeBucketAPI{headErrs: []error{errors.New("404")}}
	if err := ensureCodeBucket(context.Background(), f, "b", "us-east-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.createCalls[0].CreateBucketConfiguration != nil {
		t.Fatalf("us-east-1 must not set a LocationConstraint")
	}
}

func TestEnsureCodeBucketTolratesAlreadyOwned(t *testing.T) {
	f := &fakeCodeBucketAPI{
		headErrs:  []error{errors.New("404")},
		createErr: &s3types.BucketAlreadyOwnedByYou{},
	}
	if err := ensureCodeBucket(context.Background(), f, "b", "ap-south-1"); err != nil {
		t.Fatalf("BucketAlreadyOwnedByYou must be tolerated, got: %v", err)
	}
}

func TestDefaultCodeBucketAndKeyFormats(t *testing.T) {
	if got := defaultCodeBucket("123456789012", "ap-south-1"); got != "runfabric-code-123456789012-ap-south-1" {
		t.Fatalf("unexpected bucket name: %s", got)
	}
	key := codeObjectKey("svc", "dev", []byte("payload"))
	if !strings.HasPrefix(key, "code/svc/dev/") || !strings.HasSuffix(key, ".zip") {
		t.Fatalf("unexpected key format: %s", key)
	}
	// Content-addressed: same bytes → same key; different bytes → different key.
	if key != codeObjectKey("svc", "dev", []byte("payload")) {
		t.Fatal("key must be deterministic for identical content")
	}
	if key == codeObjectKey("svc", "dev", []byte("other")) {
		t.Fatal("key must change when content changes")
	}
}

func TestResolveCodeRefInlineUnderThreshold(t *testing.T) {
	// Small zips stay inline; the S3 client must not be touched (clients is nil).
	code, err := resolveCodeRef(context.Background(), nil, "svc", "dev", "us-east-1", []byte("tiny"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !code.inline() || string(code.zipBytes) != "tiny" {
		t.Fatalf("expected inline code ref, got %+v", code)
	}
}

func TestZipDeployDirectoryExcludesNodeModules(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "dist", "handler.js"), "exports.handler = 1")
	mustWrite(t, filepath.Join(root, "node_modules", "pkg", "index.js"), "junk")
	mustWrite(t, filepath.Join(root, ".git", "HEAD"), "ref")

	zipBytes, err := zipDeployDirectory(root)
	if err != nil {
		t.Fatalf("zip failed: %v", err)
	}
	if len(zipBytes) == 0 {
		t.Fatal("expected non-empty zip")
	}
	names := zipEntryNames(t, zipBytes)
	if _, ok := names["dist/handler.js"]; !ok {
		t.Fatalf("expected dist/handler.js in zip, got %v", names)
	}
	for name := range names {
		if strings.Contains(name, "node_modules") || strings.Contains(name, ".git") {
			t.Fatalf("zip must exclude node_modules/.git, found %s", name)
		}
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func zipEntryNames(t *testing.T, zipBytes []byte) map[string]struct{} {
	t.Helper()
	names := map[string]struct{}{}
	r, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("read zip: %v", err)
	}
	for _, f := range r.File {
		names[f.Name] = struct{}{}
	}
	return names
}

//go:build integration
// +build integration

package states

import (
	"context"
	"strings"
	"testing"

	"github.com/runfabric/runfabric/platform/state/backends"
)

func TestRegisteredLocalBundles(t *testing.T) {
	bundle, err := backends.NewBundle(context.Background(), backends.Options{
		Kind: "local",
		Root: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("NewBundle(local): %v", err)
	}
	if got := bundle.Receipts.Kind(); got != "local" {
		t.Fatalf("receipts kind=%q want local", got)
	}
}

func TestNewBundle_UnsupportedKind(t *testing.T) {
	t.Run("file", func(t *testing.T) {
		_, err := backends.NewBundle(context.Background(), backends.Options{
			Kind: "file",
			Root: t.TempDir(),
		})
		if err == nil {
			t.Fatalf("expected error for unsupported kind")
		}
		if !strings.Contains(err.Error(), "unsupported backend kind") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("unknown", func(t *testing.T) {
		_, err := backends.NewBundle(context.Background(), backends.Options{
			Kind: "does-not-exist",
			Root: t.TempDir(),
		})
		if err == nil {
			t.Fatalf("expected error for unsupported kind")
		}
		if !strings.Contains(err.Error(), "unsupported backend kind") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestNewBundle_S3RequiresBucket(t *testing.T) {
	_, err := backends.NewBundle(context.Background(), backends.Options{
		Kind: "s3",
		Root: t.TempDir(),
	})
	if err == nil {
		t.Fatalf("expected error for missing s3 bucket")
	}
	if !strings.Contains(err.Error(), "backend.s3Bucket required for kind s3") {
		t.Fatalf("unexpected error: %v", err)
	}
}

package external

import (
	"context"
	"testing"
)

func TestExternalSecretManagerAdapter_ResolveSecret(t *testing.T) {
	exe := buildStubPlugin(t)
	adapter := NewExternalSecretManagerAdapter("stub-secret-manager", exe)

	got, err := adapter.ResolveSecret(context.Background(), "aws-sm://team/prod/db/password")
	if err != nil {
		t.Fatalf("ResolveSecret error: %v", err)
	}
	want := "resolved:aws-sm://team/prod/db/password"
	if got != want {
		t.Fatalf("ResolveSecret=%q want %q", got, want)
	}
}

func TestExternalSecretManagerAdapter_ResolveSecret_RequiresReference(t *testing.T) {
	exe := buildStubPlugin(t)
	adapter := NewExternalSecretManagerAdapter("stub-secret-manager", exe)

	if _, err := adapter.ResolveSecret(context.Background(), ""); err == nil {
		t.Fatal("expected missing secret reference error")
	}
}

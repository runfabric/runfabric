package main

import (
	"context"
	"strings"
	"testing"
)

func TestParseAzureSecretRef(t *testing.T) {
	parsed, err := parseAzureSecretRef("azure-kv://my-vault/db-password/42?jsonKey=value")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.Vault != "my-vault" {
		t.Fatalf("vault=%q", parsed.Vault)
	}
	if parsed.Secret != "db-password" {
		t.Fatalf("secret=%q", parsed.Secret)
	}
	if parsed.Version != "42" {
		t.Fatalf("version=%q", parsed.Version)
	}
	if parsed.JSONKey != "value" {
		t.Fatalf("jsonKey=%q", parsed.JSONKey)
	}
}

func TestResolveSecret_UsesRunnerAndJSONKey(t *testing.T) {
	p := &plugin{
		run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if name != "az" {
				t.Fatalf("name=%q", name)
			}
			joined := strings.Join(args, " ")
			if !strings.Contains(joined, "keyvault secret show") {
				t.Fatalf("unexpected args: %v", args)
			}
			if !strings.Contains(joined, "--vault-name my-vault") {
				t.Fatalf("missing vault arg: %v", args)
			}
			if !strings.Contains(joined, "--name my-secret") {
				t.Fatalf("missing secret arg: %v", args)
			}
			if !strings.Contains(joined, "--version 7") {
				t.Fatalf("missing version arg: %v", args)
			}
			return []byte(`{"password":"top-secret"}`), nil
		},
		getenv: func(string) string { return "" },
	}
	got, err := p.ResolveSecret(context.Background(), "azure-kv://my-vault/my-secret/7?jsonKey=password")
	if err != nil {
		t.Fatalf("ResolveSecret: %v", err)
	}
	if got != "top-secret" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveSecret_VaultFromEnv(t *testing.T) {
	p := &plugin{
		run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			joined := strings.Join(args, " ")
			if !strings.Contains(joined, "--vault-name env-vault") {
				t.Fatalf("vault flag missing in %v", args)
			}
			return []byte("value-from-env"), nil
		},
		getenv: func(key string) string {
			if key == envAzureKeyVaultName {
				return "env-vault"
			}
			return ""
		},
	}
	got, err := p.ResolveSecret(context.Background(), "azure-kv://my-secret")
	if err != nil {
		t.Fatalf("ResolveSecret: %v", err)
	}
	if got != "value-from-env" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveSecret_RejectsUnsupportedScheme(t *testing.T) {
	p := newPlugin()
	_, err := p.ResolveSecret(context.Background(), "vault://secret/data/team")
	if err == nil || !strings.Contains(err.Error(), "unsupported azure key vault reference") {
		t.Fatalf("expected unsupported scheme error, got %v", err)
	}
}

package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseVaultSecretRef(t *testing.T) {
	parsed, err := parseVaultSecretRef("vault://secret/data/myapp?field=password&addr=https://vault.example.com")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.Path != "secret/data/myapp" {
		t.Fatalf("path=%q", parsed.Path)
	}
	if parsed.Field != "password" {
		t.Fatalf("field=%q", parsed.Field)
	}
	if parsed.Address != "https://vault.example.com" {
		t.Fatalf("address=%q", parsed.Address)
	}
}

func TestResolveSecret_KVV2Field(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Vault-Token"); got != "token-123" {
			t.Fatalf("token header=%q", got)
		}
		_, _ = w.Write([]byte(`{"data":{"data":{"password":"vault-secret"}}}`))
	}))
	defer server.Close()

	p := newPlugin()
	p.httpClient = server.Client()
	p.getenv = func(key string) string {
		switch key {
		case envVaultAddr:
			return server.URL
		case envVaultToken:
			return "token-123"
		default:
			return ""
		}
	}
	got, err := p.ResolveSecret(context.Background(), "vault://secret/data/myapp?field=password")
	if err != nil {
		t.Fatalf("ResolveSecret: %v", err)
	}
	if got != "vault-secret" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveSecret_DefaultValueField(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"value":"simple-secret"}}`))
	}))
	defer server.Close()

	p := newPlugin()
	p.httpClient = server.Client()
	p.getenv = func(key string) string {
		switch key {
		case envVaultAddr:
			return server.URL
		case envVaultToken:
			return "token-123"
		default:
			return ""
		}
	}
	got, err := p.ResolveSecret(context.Background(), "vault://kv/app")
	if err != nil {
		t.Fatalf("ResolveSecret: %v", err)
	}
	if got != "simple-secret" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveSecret_RequiresToken(t *testing.T) {
	p := newPlugin()
	p.getenv = func(key string) string {
		if key == envVaultAddr {
			return "https://vault.example.com"
		}
		return ""
	}
	_, err := p.ResolveSecret(context.Background(), "vault://secret/data/myapp")
	if err == nil || !strings.Contains(err.Error(), envVaultToken) {
		t.Fatalf("expected token error, got %v", err)
	}
}

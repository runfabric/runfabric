package main

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResolveSecret_EndpointOverride(t *testing.T) {
	const want = "value-from-emulator"

	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		encoded := base64.StdEncoding.EncodeToString([]byte(want))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"payload":{"data":"` + encoded + `"}}`))
	}))
	defer srv.Close()

	p := &plugin{
		run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			t.Fatalf("gcloud should not run when %s is set (name=%q)", envGCPEndpointURL, name)
			return nil, nil
		},
		getenv: func(key string) string {
			if key == envGCPEndpointURL {
				return srv.URL
			}
			return ""
		},
	}

	got, err := p.ResolveSecret(context.Background(), "gcp-sm://my-project/db-password/latest")
	if err != nil {
		t.Fatalf("ResolveSecret: %v", err)
	}
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if wantPath := "/v1/projects/my-project/secrets/db-password/versions/latest:access"; gotPath != wantPath {
		t.Fatalf("override host received path %q, want %q", gotPath, wantPath)
	}
}

func TestResolveSecret_EndpointOverrideJSONKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		encoded := base64.StdEncoding.EncodeToString([]byte(`{"password":"top-secret"}`))
		_, _ = w.Write([]byte(`{"payload":{"data":"` + encoded + `"}}`))
	}))
	defer srv.Close()

	p := &plugin{
		run:    func(ctx context.Context, name string, args ...string) ([]byte, error) { return nil, nil },
		getenv: func(key string) string { return srv.URL },
	}

	got, err := p.ResolveSecret(context.Background(), "gcp-sm://my-project/creds/latest?jsonKey=password")
	if err != nil {
		t.Fatalf("ResolveSecret: %v", err)
	}
	if got != "top-secret" {
		t.Fatalf("got %q", got)
	}
}

func TestParseGCPSecretRef(t *testing.T) {
	parsed, err := parseGCPSecretRef("gcp-sm://my-project/db-password/latest?jsonKey=value")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.Project != "my-project" {
		t.Fatalf("project=%q", parsed.Project)
	}
	if parsed.Secret != "db-password" {
		t.Fatalf("secret=%q", parsed.Secret)
	}
	if parsed.Version != "latest" {
		t.Fatalf("version=%q", parsed.Version)
	}
	if parsed.JSONKey != "value" {
		t.Fatalf("jsonKey=%q", parsed.JSONKey)
	}
}

func TestResolveSecret_UsesRunnerAndJSONKey(t *testing.T) {
	p := &plugin{
		run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if name != "gcloud" {
				t.Fatalf("name=%q", name)
			}
			joined := strings.Join(args, " ")
			if !strings.Contains(joined, "secrets versions access latest") {
				t.Fatalf("unexpected args: %v", args)
			}
			return []byte(`{"password":"top-secret"}`), nil
		},
		getenv: func(string) string { return "" },
	}
	got, err := p.ResolveSecret(context.Background(), "gcp-sm://my-project/my-secret/latest?jsonKey=password")
	if err != nil {
		t.Fatalf("ResolveSecret: %v", err)
	}
	if got != "top-secret" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveSecret_ProjectFromEnv(t *testing.T) {
	p := &plugin{
		run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			joined := strings.Join(args, " ")
			if !strings.Contains(joined, "--project env-project") {
				t.Fatalf("project flag missing in %v", args)
			}
			return []byte("value-from-env"), nil
		},
		getenv: func(key string) string {
			if key == envGCPProjectID {
				return "env-project"
			}
			return ""
		},
	}
	got, err := p.ResolveSecret(context.Background(), "gcp-sm://my-secret")
	if err != nil {
		t.Fatalf("ResolveSecret: %v", err)
	}
	if got != "value-from-env" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveSecret_RejectsUnsupportedScheme(t *testing.T) {
	p := newPlugin()
	_, err := p.ResolveSecret(context.Background(), "aws-sm://prod/db/password")
	if err == nil || !strings.Contains(err.Error(), "unsupported gcp secret reference") {
		t.Fatalf("expected unsupported scheme error, got %v", err)
	}
}

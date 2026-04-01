package main

import (
	"context"
	"strings"
	"testing"
)

func TestParseAWSSecretRef(t *testing.T) {
	parsed, err := parseAWSSecretRef("aws-sm://prod/service/db?region=us-east-1&versionStage=AWSCURRENT&jsonKey=password")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.SecretID != "prod/service/db" {
		t.Fatalf("secretID=%q", parsed.SecretID)
	}
	if parsed.Region != "us-east-1" {
		t.Fatalf("region=%q", parsed.Region)
	}
	if parsed.VersionStage != "AWSCURRENT" {
		t.Fatalf("versionStage=%q", parsed.VersionStage)
	}
	if parsed.JSONKey != "password" {
		t.Fatalf("jsonKey=%q", parsed.JSONKey)
	}
}

func TestResolveSecret_UsesFetcherAndJSONKey(t *testing.T) {
	p := &plugin{
		fetch: func(ctx context.Context, region, secretID, versionStage, versionID string) (string, error) {
			if region != "us-east-1" || secretID != "team/prod/db" || versionID != "v1" {
				t.Fatalf("unexpected fetch args: region=%q secretID=%q versionStage=%q versionID=%q", region, secretID, versionStage, versionID)
			}
			return `{"password":"s3cr3t"}`, nil
		},
		getenv: func(string) string { return "" },
	}
	got, err := p.ResolveSecret(context.Background(), "aws-sm://team/prod/db?region=us-east-1&versionId=v1&jsonKey=password")
	if err != nil {
		t.Fatalf("ResolveSecret: %v", err)
	}
	if got != "s3cr3t" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveSecret_UsesEnvRegionFallback(t *testing.T) {
	p := &plugin{
		fetch: func(ctx context.Context, region, secretID, versionStage, versionID string) (string, error) {
			if region != "ap-southeast-1" {
				t.Fatalf("region=%q", region)
			}
			return "plain", nil
		},
		getenv: func(key string) string {
			if key == envAWSRegion {
				return "ap-southeast-1"
			}
			return ""
		},
	}
	got, err := p.ResolveSecret(context.Background(), "aws-sm://team/prod/api")
	if err != nil {
		t.Fatalf("ResolveSecret: %v", err)
	}
	if got != "plain" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveSecret_RejectsUnsupportedScheme(t *testing.T) {
	p := newPlugin()
	_, err := p.ResolveSecret(context.Background(), "vault://secret/data/team")
	if err == nil || !strings.Contains(err.Error(), "unsupported aws secret reference") {
		t.Fatalf("expected unsupported scheme error, got %v", err)
	}
}

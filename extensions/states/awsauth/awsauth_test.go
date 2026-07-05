package awsauth

import (
	"context"
	"testing"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestScopedCredentialsWin(t *testing.T) {
	cfg, err := loadConfig(context.Background(), "eu-west-1", env(map[string]string{
		EnvAccessKeyID:     "AKIA_STATE",
		EnvSecretAccessKey: "state-secret",
		EnvSessionToken:    "state-session",
		EnvRegion:          "ap-south-1",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Region != "ap-south-1" {
		t.Fatalf("region = %q, want scoped ap-south-1", cfg.Region)
	}
	creds, err := cfg.Credentials.Retrieve(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if creds.AccessKeyID != "AKIA_STATE" || creds.SecretAccessKey != "state-secret" || creds.SessionToken != "state-session" {
		t.Fatalf("scoped static credentials not applied: %+v", creds)
	}
}

func TestNoScopedKeysUsesDefaultChain(t *testing.T) {
	cfg, err := loadConfig(context.Background(), "eu-west-1", env(map[string]string{}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Region != "eu-west-1" {
		t.Fatalf("region = %q, want caller-supplied eu-west-1", cfg.Region)
	}
	// Partial scoped set (access key only) must also fall back — never build a
	// static provider from half an identity.
	cfg2, err := loadConfig(context.Background(), "eu-west-1", env(map[string]string{EnvAccessKeyID: "AKIA_ONLY"}))
	if err != nil {
		t.Fatal(err)
	}
	c2, err := cfg2.Credentials.Retrieve(context.Background())
	if err == nil && c2.AccessKeyID == "AKIA_ONLY" {
		t.Fatal("partial scoped set must not become a static identity")
	}
}

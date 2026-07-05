package main

import (
	"testing"

	awscfg "github.com/aws/aws-sdk-go-v2/config"
)

func TestAWSLoadOptionsScopedIdentity(t *testing.T) {
	getenv := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}

	// Scoped keys → static provider present (options count grows).
	scoped := awsLoadOptions("us-east-1", getenv(map[string]string{
		envSMAccessKeyID:     "AKIA_SM",
		envSMSecretAccessKey: "sm-secret",
	}))
	plain := awsLoadOptions("us-east-1", getenv(map[string]string{}))
	if len(scoped) != len(plain)+1 {
		t.Fatalf("scoped identity must add a credentials option: scoped=%d plain=%d", len(scoped), len(plain))
	}

	// Verify the static identity actually lands in the load options.
	var lo awscfg.LoadOptions
	for _, opt := range scoped {
		if err := opt(&lo); err != nil {
			t.Fatal(err)
		}
	}
	if lo.Credentials == nil {
		t.Fatal("scoped credentials provider not set")
	}
	creds, err := lo.Credentials.Retrieve(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if creds.AccessKeyID != "AKIA_SM" || creds.SecretAccessKey != "sm-secret" {
		t.Fatalf("wrong scoped identity: %+v", creds)
	}

	// Partial set (secret only) must not create a static provider.
	partial := awsLoadOptions("us-east-1", getenv(map[string]string{envSMSecretAccessKey: "sm-secret"}))
	if len(partial) != len(plain) {
		t.Fatal("partial scoped set must fall back to the default chain")
	}
}

package secrets

import (
	"strings"
	"testing"
)

func TestResolveString_ConfigMap(t *testing.T) {
	got, err := ResolveString("postgres://${secret:DB_URL}", map[string]string{
		"DB_URL": "localhost:5432/app",
	}, nil)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got != "postgres://localhost:5432/app" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveString_SecretURLChain(t *testing.T) {
	got, err := ResolveString("${secret:app_db}", map[string]string{
		"app_db": "secret://DATABASE_URL",
	}, func(key string) (string, bool) {
		if key == "DATABASE_URL" {
			return "postgres://example", true
		}
		return "", false
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got != "postgres://example" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveString_MissingSecret(t *testing.T) {
	if _, err := ResolveString("${secret:MISSING}", nil, func(key string) (string, bool) { return "", false }); err == nil {
		t.Fatal("expected missing secret error")
	}
}

func TestValidateConfigSecretMap(t *testing.T) {
	if err := ValidateConfigSecretMap(map[string]string{"db": "secret://DB_URL"}); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if err := ValidateConfigSecretMap(map[string]string{"": "x"}); err == nil {
		t.Fatal("expected empty key error")
	}
	if err := ValidateConfigSecretMap(map[string]string{"db": "secret://"}); err == nil {
		t.Fatal("expected empty secret:// ref error")
	}
}

func TestResolveString_SecretManagerReference(t *testing.T) {
	resetSchemes := SetSecretManagerRefSchemes([]string{"vault"})
	defer resetSchemes()

	restore := SetReferenceResolver(func(ref string) (string, error) {
		return "resolved:" + ref, nil
	})
	defer restore()

	got, err := ResolveString("${secret:db}", map[string]string{"db": "vault://apps/team/db_password"}, nil)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got != "resolved:vault://apps/team/db_password" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveString_SecretManagerReferenceRequiresResolver(t *testing.T) {
	resetSchemes := SetSecretManagerRefSchemes([]string{"aws-sm"})
	defer resetSchemes()

	restore := SetReferenceResolver(nil)
	defer restore()

	_, err := ResolveString("${secret:db}", map[string]string{"db": "aws-sm://prod/db/password"}, nil)
	if err == nil {
		t.Fatal("expected missing secret manager resolver error")
	}
	if !strings.Contains(err.Error(), "secretManagerPlugin") {
		t.Fatalf("expected secretManagerPlugin hint, got %v", err)
	}
}

func TestIsSecretManagerRef(t *testing.T) {
	resetSchemes := SetSecretManagerRefSchemes([]string{"aws-sm", "gcp-sm", "azure-kv", "azure-keyvault", "vault"})
	defer resetSchemes()

	cases := map[string]bool{
		"aws-sm://prod/db/password":                  true,
		"gcp-sm://project/secret/latest":             true,
		"azure-kv://my-vault/db-password":            true,
		"azure-keyvault://my-vault/db-password":      true,
		"vault://secret/data/service/db-password":    true,
		"https://example.com/not-a-secret-reference": false,
	}
	for input, want := range cases {
		got := IsSecretManagerRef(input)
		if got != want {
			t.Fatalf("IsSecretManagerRef(%q)=%v want %v", input, got, want)
		}
	}
}

func TestValidateForStage(t *testing.T) {
	resetSchemes := SetSecretManagerRefSchemes([]string{"vault", "azure-kv"})
	defer resetSchemes()

	if err := ValidateForStage(map[string]string{"db": "plain-static"}, "prod"); err == nil {
		t.Fatal("expected static production secret rejection")
	}
	if err := ValidateForStage(map[string]string{"db": "${env:DB_PASSWORD}"}, "prod"); err != nil {
		t.Fatalf("expected env reference to pass: %v", err)
	}
	if err := ValidateForStage(map[string]string{"db": "vault://prod/db/password"}, "production"); err != nil {
		t.Fatalf("expected secret manager reference to pass: %v", err)
	}
	if err := ValidateForStage(map[string]string{"db": "azure-kv://prod-vault/db-password"}, "production"); err != nil {
		t.Fatalf("expected azure secret manager reference to pass: %v", err)
	}
	if err := ValidateForStage(map[string]string{"db": "plain-dev-ok"}, "dev"); err != nil {
		t.Fatalf("expected non-production stage to allow static secret: %v", err)
	}
}

func TestSecretManagerRefExamples(t *testing.T) {
	reset := SetSecretManagerRefSchemes([]string{"vault", "aws-sm"})
	defer reset()

	got := SecretManagerRefExamples()
	if !strings.Contains(got, "vault://") || !strings.Contains(got, "aws-sm://") {
		t.Fatalf("unexpected examples string: %q", got)
	}
}

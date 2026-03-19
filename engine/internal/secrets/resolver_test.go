package secrets

import "testing"

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

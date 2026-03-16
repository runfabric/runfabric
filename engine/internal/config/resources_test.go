package config

import (
	"os"
	"testing"
)

func TestResolveResourceBindings_Empty(t *testing.T) {
	out, err := ResolveResourceBindings(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out != nil {
		t.Fatalf("expected nil, got %v", out)
	}

	cfg := &Config{Resources: map[string]any{}}
	out, err = ResolveResourceBindings(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out != nil {
		t.Fatalf("expected nil, got %v", out)
	}
}

func TestResolveResourceBindings_ConnectionStringEnv(t *testing.T) {
	os.Setenv("TEST_DATABASE_URL", "postgres://localhost/db")
	defer os.Unsetenv("TEST_DATABASE_URL")

	cfg := &Config{
		Resources: map[string]any{
			"db": map[string]any{
				"type":                "database",
				"envVar":              "DATABASE_URL",
				"connectionStringEnv": "TEST_DATABASE_URL",
			},
		},
	}
	out, err := ResolveResourceBindings(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out["DATABASE_URL"] != "postgres://localhost/db" {
		t.Fatalf("expected DATABASE_URL=postgres://localhost/db, got %q", out["DATABASE_URL"])
	}
}

func TestResolveResourceBindings_ConnectionStringLiteral(t *testing.T) {
	cfg := &Config{
		Resources: map[string]any{
			"cache": map[string]any{
				"type":             "cache",
				"envVar":           "REDIS_URL",
				"connectionString": "redis://localhost:6379",
			},
		},
	}
	out, err := ResolveResourceBindings(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out["REDIS_URL"] != "redis://localhost:6379" {
		t.Fatalf("expected REDIS_URL=redis://localhost:6379, got %q", out["REDIS_URL"])
	}
}

func TestResolveResourceBindings_ConnectionStringEnvRef(t *testing.T) {
	os.Setenv("MY_REDIS", "redis://cache.example.com")
	defer os.Unsetenv("MY_REDIS")

	cfg := &Config{
		Resources: map[string]any{
			"cache": map[string]any{
				"envVar":           "REDIS_URL",
				"connectionString": "${env:MY_REDIS}",
			},
		},
	}
	out, err := ResolveResourceBindings(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out["REDIS_URL"] != "redis://cache.example.com" {
		t.Fatalf("expected REDIS_URL=redis://cache.example.com, got %q", out["REDIS_URL"])
	}
}

func TestResolveResourceBindings_SkipsMissingEnvVar(t *testing.T) {
	cfg := &Config{
		Resources: map[string]any{
			"db": map[string]any{
				"type":                "database",
				"connectionStringEnv": "DATABASE_URL",
				// no envVar -> skipped
			},
		},
	}
	out, err := ResolveResourceBindings(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Fatalf("expected no bindings, got %v", out)
	}
}

func TestEnvVarToResourceKey(t *testing.T) {
	if out := EnvVarToResourceKey(nil); out != nil {
		t.Fatalf("nil config: expected nil, got %v", out)
	}
	cfg := &Config{
		Resources: map[string]any{
			"db":    map[string]any{"envVar": "DATABASE_URL", "connectionString": "x"},
			"cache": map[string]any{"envVar": "REDIS_URL", "connectionString": "y"},
		},
	}
	out := EnvVarToResourceKey(cfg)
	if out["DATABASE_URL"] != "db" || out["REDIS_URL"] != "cache" {
		t.Fatalf("expected db->DATABASE_URL, cache->REDIS_URL; got %v", out)
	}
}

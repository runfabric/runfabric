package config

import (
	"testing"
)

func TestAddonCatalog(t *testing.T) {
	catalog := AddonCatalog()
	if len(catalog) == 0 {
		t.Fatal("AddonCatalog() should return at least one entry")
	}
	seen := make(map[string]bool)
	for _, e := range catalog {
		if e.Name == "" {
			t.Errorf("catalog entry has empty name")
		}
		if seen[e.Name] {
			t.Errorf("duplicate catalog name %q", e.Name)
		}
		seen[e.Name] = true
	}
}

func TestResolveAddonBindings_Empty(t *testing.T) {
	env, err := ResolveAddonBindings(nil)
	if err != nil || env != nil {
		t.Fatalf("nil config: want nil,nil got %v,%v", env, err)
	}
	env, err = ResolveAddonBindings(&Config{})
	if err != nil || env != nil {
		t.Fatalf("empty addons: want nil,nil got %v,%v", env, err)
	}
}

func TestResolveAddonBindings_FromSecrets(t *testing.T) {
	// addon secret ref can be key into config.Secrets
	cfg := &Config{
		Secrets: map[string]string{"sentry_dsn": "${env:SENTRY_DSN}"},
		Addons: map[string]AddonConfig{
			"sentry": {
				Secrets: map[string]string{"SENTRY_DSN": "sentry_dsn"},
			},
		},
	}
	t.Setenv("SENTRY_DSN", "https://key@o1.ingest.sentry.io/2")
	env, err := ResolveAddonBindings(cfg)
	if err != nil {
		t.Fatalf("ResolveAddonBindings: %v", err)
	}
	if env["SENTRY_DSN"] != "https://key@o1.ingest.sentry.io/2" {
		t.Errorf("SENTRY_DSN: got %q", env["SENTRY_DSN"])
	}
}

func TestValidateAddons_EmptyKey(t *testing.T) {
	cfg := &Config{
		Addons: map[string]AddonConfig{
			"x": {Name: "", Secrets: map[string]string{"K": "v"}},
		},
	}
	Normalize(cfg)
	if err := ValidateAddons(cfg); err != nil {
		t.Errorf("ValidateAddons: %v", err)
	}
}

func TestValidateAddons_EmptySecretKey(t *testing.T) {
	cfg := &Config{
		Addons: map[string]AddonConfig{
			"x": {Secrets: map[string]string{"": "v"}},
		},
	}
	if err := ValidateAddons(cfg); err == nil {
		t.Error("ValidateAddons: expected error for empty env var name")
	}
}

func TestResolveAddonBindingsForKeys(t *testing.T) {
	t.Setenv("SENTRY_DSN", "https://sentry.example/1")
	t.Setenv("DD_API_KEY", "dd-key")
	cfg := &Config{
		Addons: map[string]AddonConfig{
			"sentry":  {Secrets: map[string]string{"SENTRY_DSN": "${env:SENTRY_DSN}"}},
			"datadog": {Secrets: map[string]string{"DD_API_KEY": "${env:DD_API_KEY}"}},
		},
	}
	// All keys: both env vars
	env, err := ResolveAddonBindingsForKeys(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if env["SENTRY_DSN"] != "https://sentry.example/1" || env["DD_API_KEY"] != "dd-key" {
		t.Errorf("all keys: got %v", env)
	}
	// Only sentry
	env, err = ResolveAddonBindingsForKeys(cfg, []string{"sentry"})
	if err != nil {
		t.Fatal(err)
	}
	if env["SENTRY_DSN"] != "https://sentry.example/1" {
		t.Errorf("sentry only: SENTRY_DSN got %q", env["SENTRY_DSN"])
	}
	if _, ok := env["DD_API_KEY"]; ok {
		t.Error("sentry only: should not have DD_API_KEY")
	}
}

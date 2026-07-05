package provider

import "testing"

func TestResolveVar(t *testing.T) {
	vars := []CredentialVar{
		{EnvKey: "RF_TEST_ROUTER_TOKEN", Fallback: "RF_TEST_PROVIDER_TOKEN"},
		{EnvKey: "RF_TEST_PLAIN"},
	}

	t.Run("own key wins over fallback", func(t *testing.T) {
		t.Setenv("RF_TEST_ROUTER_TOKEN", "own")
		t.Setenv("RF_TEST_PROVIDER_TOKEN", "fallback")
		if got := ResolveVar(vars, "RF_TEST_ROUTER_TOKEN"); got != "own" {
			t.Fatalf("got %q, want own", got)
		}
	})

	t.Run("fallback used when own key unset", func(t *testing.T) {
		t.Setenv("RF_TEST_ROUTER_TOKEN", "")
		t.Setenv("RF_TEST_PROVIDER_TOKEN", "fallback")
		if got := ResolveVar(vars, "RF_TEST_ROUTER_TOKEN"); got != "fallback" {
			t.Fatalf("got %q, want fallback", got)
		}
	})

	t.Run("empty when neither set", func(t *testing.T) {
		t.Setenv("RF_TEST_ROUTER_TOKEN", "")
		t.Setenv("RF_TEST_PROVIDER_TOKEN", "")
		if got := ResolveVar(vars, "RF_TEST_ROUTER_TOKEN"); got != "" {
			t.Fatalf("got %q, want empty", got)
		}
	})

	t.Run("no fallback declared", func(t *testing.T) {
		t.Setenv("RF_TEST_PLAIN", "")
		if got := ResolveVar(vars, "RF_TEST_PLAIN"); got != "" {
			t.Fatalf("got %q, want empty", got)
		}
	})

	t.Run("undeclared key resolves empty", func(t *testing.T) {
		t.Setenv("RF_TEST_UNDECLARED", "value")
		if got := ResolveVar(vars, "RF_TEST_UNDECLARED"); got != "" {
			t.Fatalf("undeclared key must not resolve, got %q", got)
		}
	})
}

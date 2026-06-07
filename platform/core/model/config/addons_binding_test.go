package config

import "testing"

func TestResolveAddonBindings_ConflictingEnvVarErrors(t *testing.T) {
	cfg := &Config{
		Service: "svc",
		Secrets: map[string]string{"a": "valueA", "b": "valueB"},
		Addons: map[string]AddonConfig{
			"x": {Secrets: map[string]string{"API_KEY": "a"}},
			"y": {Secrets: map[string]string{"API_KEY": "b"}},
		},
	}
	if _, err := ResolveAddonBindings(cfg); err == nil {
		t.Fatal("expected error for two addons binding API_KEY to different values")
	}
}

func TestResolveAddonBindings_SameValueIsDeterministic(t *testing.T) {
	cfg := &Config{
		Service: "svc",
		Secrets: map[string]string{"a": "same", "b": "same"},
		Addons: map[string]AddonConfig{
			"x": {Secrets: map[string]string{"API_KEY": "a"}},
			"y": {Secrets: map[string]string{"API_KEY": "b"}},
		},
	}
	// Run repeatedly: map iteration order varies, but the result must be stable.
	for i := 0; i < 20; i++ {
		out, err := ResolveAddonBindings(cfg)
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		if out["API_KEY"] != "same" {
			t.Fatalf("iteration %d: API_KEY = %q, want same", i, out["API_KEY"])
		}
	}
}

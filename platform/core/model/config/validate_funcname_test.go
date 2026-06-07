package config

import "testing"

func TestValidate_RejectsUnsafeFunctionNames(t *testing.T) {
	base := func(name string) *Config {
		return &Config{
			Service:  "svc",
			Provider: ProviderConfig{Name: "aws-lambda", Runtime: "nodejs"},
			Functions: map[string]FunctionConfig{
				name: {Handler: "src/handler"},
			},
		}
	}

	bad := []string{
		"../../etc/cron.d/x",
		"a/b",
		"a;rm -rf /",
		"a b",
		"..",
		"-leadingdash", // must start alphanumeric
		"",
	}
	for _, name := range bad {
		if err := Validate(base(name)); err == nil {
			t.Errorf("expected validation error for unsafe function name %q", name)
		}
	}

	good := []string{"api", "my-fn", "fn_2", "Worker.v1"}
	for _, name := range good {
		if err := Validate(base(name)); err != nil {
			t.Errorf("valid function name %q rejected: %v", name, err)
		}
	}
}

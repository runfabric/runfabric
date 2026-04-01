package extensions

import "testing"

func TestParsePluginKindFlag(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{name: "provider", raw: "provider"},
		{name: "runtime", raw: "runtime"},
		{name: "simulator", raw: "simulator"},
		{name: "router", raw: "router"},
		{name: "secret-manager", raw: "secret-manager"},
		{name: "state", raw: "state"},
		{name: "invalid", raw: "providers", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parsePluginKindFlag(tt.raw)
			if tt.wantErr && err == nil {
				t.Fatal("expected parse error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected parse error: %v", err)
			}
		})
	}
}

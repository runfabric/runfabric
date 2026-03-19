package app

import "testing"

func TestValidateServiceScope(t *testing.T) {
	tests := []struct {
		name        string
		config      string
		requested   string
		expectError bool
	}{
		{name: "empty requested", config: "svc", requested: "", expectError: false},
		{name: "exact match", config: "svc", requested: "svc", expectError: false},
		{name: "mismatch", config: "svc", requested: "other", expectError: true},
		{name: "config empty with request", config: "", requested: "svc", expectError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateServiceScope(tt.config, tt.requested)
			if tt.expectError && err == nil {
				t.Fatalf("expected error")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
		})
	}
}

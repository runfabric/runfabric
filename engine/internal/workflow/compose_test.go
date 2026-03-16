package workflow

import (
	"reflect"
	"testing"
)

func TestServiceBindingEnv(t *testing.T) {
	tests := []struct {
		name   string
		input  map[string]string
		expect map[string]string
	}{
		{"empty", map[string]string{}, map[string]string{}},
		{"single", map[string]string{"api": "https://api.example.com"}, map[string]string{"SERVICE_API_URL": "https://api.example.com"}},
		{"multi", map[string]string{"api": "https://a", "worker": "https://w"},
			map[string]string{"SERVICE_API_URL": "https://a", "SERVICE_WORKER_URL": "https://w"}},
		{"hyphen", map[string]string{"my-service": "https://x"}, map[string]string{"SERVICE_MY_SERVICE_URL": "https://x"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ServiceBindingEnv(tt.input)
			if !reflect.DeepEqual(got, tt.expect) {
				t.Errorf("ServiceBindingEnv() = %v, want %v", got, tt.expect)
			}
		})
	}
}

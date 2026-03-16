package app

import (
	"testing"
)

func TestServiceURLFromReceipt(t *testing.T) {
	tests := []struct {
		name    string
		outputs map[string]string
		want    string
	}{
		{"nil", nil, ""},
		{"empty", map[string]string{}, ""},
		{"ServiceURL", map[string]string{"ServiceURL": "https://api.example.com"}, "https://api.example.com"},
		{"url", map[string]string{"url": "https://x"}, "https://x"},
		{"ApiUrl", map[string]string{"ApiUrl": "https://a"}, "https://a"},
		{"prefer ServiceURL", map[string]string{"url": "https://u", "ServiceURL": "https://s"}, "https://s"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ServiceURLFromReceipt(tt.outputs)
			if got != tt.want {
				t.Errorf("ServiceURLFromReceipt() = %q, want %q", got, tt.want)
			}
		})
	}
}

package external

import (
	"net"
	"testing"
)

func TestIsPublicIP(t *testing.T) {
	cases := []struct {
		ip            string
		allowLoopback bool
		want          bool
	}{
		{"8.8.8.8", false, true},
		{"1.1.1.1", false, true},
		{"169.254.169.254", false, false}, // cloud metadata (link-local)
		{"10.0.0.1", false, false},        // private
		{"192.168.1.1", false, false},     // private
		{"172.16.0.1", false, false},      // private
		{"127.0.0.1", false, false},       // loopback disallowed
		{"127.0.0.1", true, true},         // loopback allowed for local dev
		{"::1", false, false},             // ipv6 loopback
		{"fd00::1", false, false},         // ipv6 ULA (private)
		{"0.0.0.0", false, false},         // unspecified
	}
	for _, c := range cases {
		ip := net.ParseIP(c.ip)
		if ip == nil {
			t.Fatalf("bad test ip %q", c.ip)
		}
		if got := isPublicIP(ip, c.allowLoopback); got != c.want {
			t.Errorf("isPublicIP(%s, allowLoopback=%v) = %v, want %v", c.ip, c.allowLoopback, got, c.want)
		}
	}
}

func TestSecureTransportBlocksPrivateDial(t *testing.T) {
	tr := secureTransport(false)
	// Dial an already-resolved private address: must be refused by the guard.
	_, err := tr.DialContext(t.Context(), "tcp", "169.254.169.254:80")
	if err == nil {
		t.Fatal("expected SSRF guard to refuse dialing cloud metadata address")
	}
}

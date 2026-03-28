package routers

import "testing"

func TestBuiltinRouterRegistry_IncludesExpansionRouters(t *testing.T) {
	reg := NewBuiltinRegistry()
	for _, id := range []string{"cloudflare", "route53", "ns1", "azure-traffic-manager"} {
		if _, err := reg.Get(id); err != nil {
			t.Fatalf("expected builtin router %q to be registered: %v", id, err)
		}
	}
}

package unit

import (
	"testing"

	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy"
)

func TestRegistryGet(t *testing.T) {
	ids := providerpolicy.BuiltinImplementationIDs()
	if len(ids) == 0 {
		t.Skip("no builtin providers defined in policy")
	}
	id := ids[0]

	reg := providers.NewRegistry()
	if err := reg.Register(resolveBuiltinProvider(t)); err != nil {
		t.Fatalf("register provider: %v", err)
	}

	p, ok := reg.Get(id)
	if !ok {
		t.Fatalf("expected provider %q after registration", id)
	}

	if p.Meta().Name != id {
		t.Fatalf("unexpected provider name: got %q, want %q", p.Meta().Name, id)
	}
}

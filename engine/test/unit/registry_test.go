package unit

import (
	"testing"

	"github.com/runfabric/runfabric/engine/internal/providers"
	awsprovider "github.com/runfabric/runfabric/engine/providers/aws"
)

func TestRegistryGet(t *testing.T) {
	reg := providers.NewRegistry()
	reg.Register(awsprovider.New())

	p, err := reg.Get("aws")
	if err != nil {
		t.Fatalf("expected provider, got error: %v", err)
	}

	if p.Name() != "aws" {
		t.Fatalf("unexpected provider name: %s", p.Name())
	}
}

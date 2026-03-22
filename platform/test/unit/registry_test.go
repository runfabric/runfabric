package unit

import (
	"testing"

	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	awsprovider "github.com/runfabric/runfabric/platform/extensions/interfaces/providers/aws"
)

func TestRegistryGet(t *testing.T) {
	reg := providers.NewRegistry()
	if err := reg.Register(awsprovider.New()); err != nil {
		t.Fatalf("register provider: %v", err)
	}

	p, ok := reg.Get("aws-lambda")
	if !ok {
		t.Fatal("expected provider")
	}

	if p.Meta().Name != "aws-lambda" {
		t.Fatalf("unexpected provider name: %s", p.Meta().Name)
	}
}

package providers

import "testing"

func TestNewDefaultProviderCapabilityCatalog_ListProviders(t *testing.T) {
	catalog, err := NewDefaultProviderCapabilityCatalog()
	if err != nil {
		t.Fatalf("create provider capability catalog: %v", err)
	}
	providers, err := catalog.ListProviders()
	if err != nil {
		t.Fatalf("list providers: %v", err)
	}
	if len(providers) == 0 {
		t.Fatal("expected at least one provider in capability catalog")
	}
}

func TestNewDefaultProviderCapabilityCatalog_SupportsTrigger(t *testing.T) {
	catalog, err := NewDefaultProviderCapabilityCatalog()
	if err != nil {
		t.Fatalf("create provider capability catalog: %v", err)
	}
	ok, err := catalog.SupportsTrigger("gcp-functions", "http")
	if err != nil {
		t.Fatalf("supports trigger query: %v", err)
	}
	if !ok {
		t.Fatal("expected gcp-functions to support http trigger")
	}
}

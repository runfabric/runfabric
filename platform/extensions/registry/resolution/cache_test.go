package resolution

import "testing"

func TestNewCached_ReusesBoundaryForSameOptions(t *testing.T) {
	a, err := NewCached(Options{IncludeExternal: false})
	if err != nil {
		t.Fatalf("new cached a: %v", err)
	}
	b, err := NewCached(Options{IncludeExternal: false})
	if err != nil {
		t.Fatalf("new cached b: %v", err)
	}
	if a != b {
		t.Fatal("expected cached boundary pointer reuse")
	}
}

func TestNewCached_DifferentPinnedVersionsUseDifferentEntries(t *testing.T) {
	a, err := NewCached(Options{IncludeExternal: false, PinnedVersions: map[string]string{"vercel": "1.0.0"}})
	if err != nil {
		t.Fatalf("new cached a: %v", err)
	}
	b, err := NewCached(Options{IncludeExternal: false, PinnedVersions: map[string]string{"vercel": "2.0.0"}})
	if err != nil {
		t.Fatalf("new cached b: %v", err)
	}
	if a == b {
		t.Fatal("expected different cache entries for different pinned versions")
	}
}

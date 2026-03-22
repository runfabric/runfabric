package resolution

import "testing"

func BenchmarkResolveProvider(b *testing.B) {
	boundary, err := NewCached(Options{IncludeExternal: false})
	if err != nil {
		b.Fatalf("new cached boundary: %v", err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := boundary.ResolveProvider("vercel"); err != nil {
			b.Fatalf("resolve provider: %v", err)
		}
	}
}

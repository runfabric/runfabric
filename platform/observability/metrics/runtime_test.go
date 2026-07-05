package metrics

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRuntimeMetricsRenderOnScrape(t *testing.T) {
	reg := NewRegistry()
	EnableRuntimeMetrics(reg, "1.2.3")

	rec := httptest.NewRecorder()
	reg.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	body := rec.Body.String()

	for _, want := range []string{
		`runfabric_build_info{go_version="`,
		`version="1.2.3"`,
		"runfabric_process_goroutines",
		"runfabric_process_uptime_seconds",
		"runfabric_process_memory_heap_alloc_bytes",
		"runfabric_process_gc_cycles",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("scrape output missing %q:\n%s", want, body)
		}
	}
}

func TestRegisterCollectorReplacesByName(t *testing.T) {
	reg := NewRegistry()
	calls := 0
	reg.RegisterCollector("x", func() { calls++ })
	reg.RegisterCollector("x", func() { calls += 10 })
	reg.runCollectors()
	if calls != 10 {
		t.Fatalf("same-name registration must replace, got calls=%d", calls)
	}
}

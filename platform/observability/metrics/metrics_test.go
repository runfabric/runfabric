package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCounterGaugeRender(t *testing.T) {
	r := NewRegistry()
	r.IncCounter("reqs_total", "Total requests.", map[string]string{"route": "/deploy"})
	r.IncCounter("reqs_total", "Total requests.", map[string]string{"route": "/deploy"})
	r.SetGauge("in_flight", "In flight.", nil, 3)

	out := r.Render()
	if !strings.Contains(out, "# TYPE reqs_total counter") {
		t.Fatalf("missing counter TYPE line:\n%s", out)
	}
	if !strings.Contains(out, `reqs_total{route="/deploy"} 2`) {
		t.Fatalf("counter not aggregated:\n%s", out)
	}
	if !strings.Contains(out, "in_flight 3") {
		t.Fatalf("gauge not rendered:\n%s", out)
	}
}

func TestHistogramRender(t *testing.T) {
	r := NewRegistry()
	buckets := []float64{0.1, 0.5, 1}
	r.Observe("dur_seconds", "Duration.", buckets, nil, 0.05) // <=0.1
	r.Observe("dur_seconds", "Duration.", buckets, nil, 0.4)  // <=0.5
	r.Observe("dur_seconds", "Duration.", buckets, nil, 2.0)  // only +Inf

	out := r.Render()
	for _, want := range []string{
		`dur_seconds_bucket{le="0.1"} 1`,
		`dur_seconds_bucket{le="0.5"} 2`,
		`dur_seconds_bucket{le="1"} 2`,
		`dur_seconds_bucket{le="+Inf"} 3`,
		`dur_seconds_count 3`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

func TestHTTPMiddlewareRecords(t *testing.T) {
	r := NewRegistry()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /deploy", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	h := HTTPMiddleware(r, mux)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/deploy", nil))

	out := r.Render()
	if !strings.Contains(out, `runfabric_http_requests_total{method="GET",route="/deploy",status="201"} 1`) {
		t.Fatalf("request not recorded with matched route/status:\n%s", out)
	}
	if !strings.Contains(out, "runfabric_http_request_duration_seconds_count") {
		t.Fatalf("duration histogram not recorded:\n%s", out)
	}
}

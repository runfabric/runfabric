package daemoncmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"

	"github.com/runfabric/runfabric/internal/cli/project"
)

// scaffoldInline returns the runfabric.yml + entry-handler contents for a fresh
// project, so the invoke-local tests exercise a real engine-generated config.
func scaffoldInline(t *testing.T, lang string) (yaml, handler string) {
	t.Helper()
	res, err := project.Scaffold(project.ScaffoldOptions{
		Provider:     "aws-lambda",
		Template:     "http",
		Lang:         lang,
		StateBackend: "local",
		Service:      "invokelocaltest",
	})
	if err != nil {
		t.Fatalf("scaffold: %v", err)
	}
	// Entry is the handler ref (e.g. "src/handler.handler"); the runnable file is
	// the sole .js source in the generated set.
	for _, f := range res.Files {
		if f.Path == "runfabric.yml" {
			yaml = f.Content
		}
		if strings.HasSuffix(f.Path, ".js") {
			handler = f.Content
		}
	}
	if yaml == "" || handler == "" {
		t.Fatalf("scaffold missing runfabric.yml or .js handler (entry=%q)", res.Entry)
	}
	return yaml, handler
}

func postInvokeLocal(t *testing.T, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/invoke-local", bytes.NewReader(raw))
	rr := httptest.NewRecorder()
	handleInvokeLocal(rr, req)
	return rr
}

// TestHandleInvokeLocal_ExecutesNodeHandler runs a scaffolded JS handler through
// the simulator with no deploy and asserts the handler's real response comes back.
func TestHandleInvokeLocal_ExecutesNodeHandler(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not on PATH; skipping local-execution test")
	}
	yaml, handler := scaffoldInline(t, "js")

	rr := postInvokeLocal(t, map[string]any{
		"runfabricYaml": yaml,
		"handlerCode":   handler,
		"request":       map[string]any{"method": "GET", "path": "/"},
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	var out struct {
		OK         bool            `json:"ok"`
		Function   string          `json:"function"`
		Simulated  bool            `json:"simulated"`
		StatusCode int             `json:"statusCode"`
		Body       json.RawMessage `json:"body"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v; body=%s", err, rr.Body.String())
	}
	if !out.OK {
		t.Fatalf("ok=false; body=%s", rr.Body.String())
	}
	if !out.Simulated {
		t.Errorf("simulated=false for a node handler; want true")
	}
	if out.StatusCode != http.StatusOK {
		t.Errorf("handler statusCode = %d, want 200", out.StatusCode)
	}
	// The scaffolded HTTP handler greets from RunFabric — proves real execution
	// (not the echo fallback, which would carry method/path metadata instead).
	if !strings.Contains(string(out.Body), "RunFabric") {
		t.Errorf("handler body missing expected payload; got %s", string(out.Body))
	}
}

func TestHandleInvokeLocal_MissingYaml(t *testing.T) {
	rr := postInvokeLocal(t, map[string]any{"handlerCode": "exports.handler = async () => ({});"})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "runfabricYaml is required") {
		t.Errorf("unexpected error body: %s", rr.Body.String())
	}
}

func TestHandleInvokeLocal_UnknownFunction(t *testing.T) {
	yaml, handler := scaffoldInline(t, "js")
	rr := postInvokeLocal(t, map[string]any{
		"runfabricYaml": yaml,
		"handlerCode":   handler,
		"function":      "does-not-exist",
	})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "not found") {
		t.Errorf("unexpected error body: %s", rr.Body.String())
	}
}

func TestHandleInvokeLocal_InvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/invoke-local", strings.NewReader("{not json"))
	rr := httptest.NewRecorder()
	handleInvokeLocal(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rr.Code, rr.Body.String())
	}
}

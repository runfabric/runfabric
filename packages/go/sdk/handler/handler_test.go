package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFunc(t *testing.T) {
	called := false
	h := Func(func(event map[string]any, runCtx *Context) map[string]any {
		called = true
		if runCtx.Stage != "dev" {
			t.Errorf("stage want dev got %s", runCtx.Stage)
		}
		return map[string]any{"ok": true, "stage": runCtx.Stage}
	})
	out, err := h(context.Background(), map[string]any{"x": 1}, &Context{Stage: "dev"})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("handler not called")
	}
	if out["ok"] != true || out["stage"] != "dev" {
		t.Errorf("unexpected out: %v", out)
	}
}

func TestHTTPHandler(t *testing.T) {
	h := HTTPHandler(Func(func(event map[string]any, runCtx *Context) map[string]any {
		return map[string]any{"message": "hello", "stage": runCtx.Stage}
	}))
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"world"}`))
	req.Header.Set("X-Runfabric-Function", "api")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("code want 200 got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "hello") || !strings.Contains(rec.Body.String(), "dev") {
		t.Errorf("body: %s", rec.Body.String())
	}
}

func TestHTTPHandler_emptyBody(t *testing.T) {
	h := HTTPHandler(Func(func(event map[string]any, _ *Context) map[string]any {
		return map[string]any{"event": event}
	}))
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("code want 200 got %d", rec.Code)
	}
}

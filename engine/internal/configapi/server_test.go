package configapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidate_Valid(t *testing.T) {
	srv := NewServer("dev")
	handler := srv.Handler()
	body := []byte(`service: test
provider:
  name: aws-lambda
  runtime: nodejs
functions:
  api:
    handler: index.handler
`)
	req := httptest.NewRequest(http.MethodPost, "/validate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/yaml")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("POST /validate: status %d, body %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["ok"] != true {
		t.Errorf("expected ok=true, got %v", out["ok"])
	}
}

func TestValidate_Invalid(t *testing.T) {
	srv := NewServer("dev")
	handler := srv.Handler()
	body := []byte(`service: test
provider:
  name: ""
  runtime: nodejs
functions: {}
`)
	req := httptest.NewRequest(http.MethodPost, "/validate", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity && rec.Code != http.StatusBadRequest {
		t.Errorf("POST /validate invalid: expected 422 or 400, got %d", rec.Code)
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["ok"] != false {
		t.Errorf("expected ok=false, got %v", out["ok"])
	}
}

func TestResolve_Valid(t *testing.T) {
	srv := NewServer("dev")
	handler := srv.Handler()
	body := []byte(`service: test
provider:
  name: aws-lambda
  runtime: nodejs
functions:
  api:
    handler: index.handler
`)
	req := httptest.NewRequest(http.MethodPost, "/resolve?stage=prod", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("POST /resolve: status %d, body %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["ok"] != true {
		t.Errorf("expected ok=true, got %v", out["ok"])
	}
	if out["stage"] != "prod" {
		t.Errorf("expected stage=prod, got %v", out["stage"])
	}
	if out["config"] == nil {
		t.Error("expected config in response")
	}
}

func TestResolve_DefaultStage(t *testing.T) {
	srv := NewServer("staging")
	handler := srv.Handler()
	body := []byte(`service: test
provider:
  name: aws-lambda
  runtime: nodejs
functions:
  api:
    handler: index.handler
`)
	req := httptest.NewRequest(http.MethodPost, "/resolve", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /resolve: status %d", rec.Code)
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["stage"] != "staging" {
		t.Errorf("expected default stage=staging, got %v", out["stage"])
	}
}

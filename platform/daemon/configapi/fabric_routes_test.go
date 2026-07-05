package configapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// stubConnector records calls so route → connector wiring is testable without
// real deploys.
type stubConnector struct {
	calls []string
	fail  bool
}

func (s *stubConnector) note(format string, args ...any) {
	s.calls = append(s.calls, fmt.Sprintf(format, args...))
}
func (s *stubConnector) err() error {
	if s.fail {
		return fmt.Errorf("boom")
	}
	return nil
}
func (s *stubConnector) Validate(c, st string) error { s.note("validate:%s:%s", c, st); return s.err() }
func (s *stubConnector) Resolve(c, st string) (*ResolveResponse, error) {
	s.note("resolve:%s:%s", c, st)
	return &ResolveResponse{Payload: []byte(`{}`)}, s.err()
}
func (s *stubConnector) Plan(c, st, p string) (*PlanResponse, error) {
	s.note("plan:%s:%s:%s", c, st, p)
	return &PlanResponse{Payload: []byte(`{}`)}, s.err()
}
func (s *stubConnector) Deploy(c, st, p string) (*DeployResponse, error) {
	s.note("deploy:%s:%s:%s", c, st, p)
	return &DeployResponse{Payload: []byte(`{}`)}, s.err()
}
func (s *stubConnector) Remove(c, st, p string) (*RemoveResponse, error) {
	s.note("remove:%s:%s:%s", c, st, p)
	return &RemoveResponse{Payload: []byte(`{}`)}, s.err()
}
func (s *stubConnector) Releases(c string) (*ReleasesResponse, error) {
	s.note("releases:%s", c)
	return &ReleasesResponse{Payload: []byte(`{}`)}, s.err()
}
func (s *stubConnector) ReleaseHistory(c, st string) (*ReleasesResponse, error) {
	s.note("history:%s:%s", c, st)
	return &ReleasesResponse{Payload: []byte(`{}`)}, s.err()
}
func (s *stubConnector) FabricDeploy(c, st string) (*FabricDeployResponse, error) {
	s.note("fabric:%s:%s", c, st)
	if s.fail {
		return nil, s.err()
	}
	return &FabricDeployResponse{Payload: []byte(`{"endpoints":[{"provider":"aws"},{"provider":"gcp"}]}`)}, nil
}
func (s *stubConnector) RouterSync(c, st string, dryRun bool) (*RouterSyncResponse, error) {
	s.note("router:%s:%s:%t", c, st, dryRun)
	if s.fail {
		return nil, s.err()
	}
	return &RouterSyncResponse{Payload: []byte(`{"routing":{"strategy":"failover"}}`)}, nil
}

func newStubServer() (*Server, *stubConnector) {
	stub := &stubConnector{}
	srv := NewServer("dev")
	srv.core = stub
	return srv, stub
}

func do(t *testing.T, srv *Server, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

func TestProviderParamFlowsToLifecycleOps(t *testing.T) {
	srv, stub := newStubServer()

	if rec := do(t, srv, "POST", "/deploy?stage=prod&provider=gcp"); rec.Code != 200 {
		t.Fatalf("deploy status %d: %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, srv, "POST", "/plan?stage=prod&provider=aws"); rec.Code != 200 {
		t.Fatalf("plan status %d", rec.Code)
	}
	if rec := do(t, srv, "POST", "/remove?stage=prod"); rec.Code != 200 {
		t.Fatalf("remove status %d", rec.Code)
	}

	want := []string{
		"deploy:runfabric.yml:prod:gcp",
		"plan:runfabric.yml:prod:aws",
		"remove:runfabric.yml:prod:", // no provider → config default
	}
	if strings.Join(stub.calls, "|") != strings.Join(want, "|") {
		t.Fatalf("calls = %v, want %v", stub.calls, want)
	}
}

func TestFabricDeployAndRouterSyncRoutes(t *testing.T) {
	srv, stub := newStubServer()

	rec := do(t, srv, "POST", "/fabric/deploy?stage=prod&config=t1/runfabric.yml")
	if rec.Code != 200 {
		t.Fatalf("fabric/deploy status %d: %s", rec.Code, rec.Body.String())
	}
	var fab struct {
		Endpoints []struct {
			Provider string `json:"provider"`
		} `json:"endpoints"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &fab); err != nil || len(fab.Endpoints) != 2 {
		t.Fatalf("fabric payload not passed through verbatim: %s (%v)", rec.Body.String(), err)
	}

	if rec := do(t, srv, "POST", "/router/sync?stage=prod&dryRun=true"); rec.Code != 200 {
		t.Fatalf("router/sync status %d: %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, srv, "POST", "/router/sync?stage=prod"); rec.Code != 200 {
		t.Fatalf("router/sync status %d", rec.Code)
	}

	want := []string{
		"fabric:t1/runfabric.yml:prod",
		"router:runfabric.yml:prod:true",
		"router:runfabric.yml:prod:false",
	}
	if strings.Join(stub.calls, "|") != strings.Join(want, "|") {
		t.Fatalf("calls = %v, want %v", stub.calls, want)
	}
}

func TestFabricRoutesSurfaceErrorsAs400(t *testing.T) {
	srv, stub := newStubServer()
	stub.fail = true
	if rec := do(t, srv, "POST", "/fabric/deploy"); rec.Code != http.StatusBadRequest {
		t.Fatalf("fabric/deploy status = %d, want 400", rec.Code)
	}
	if rec := do(t, srv, "POST", "/router/sync"); rec.Code != http.StatusBadRequest {
		t.Fatalf("router/sync status = %d, want 400", rec.Code)
	}
}

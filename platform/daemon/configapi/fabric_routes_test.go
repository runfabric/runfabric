package configapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
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
func (s *stubConnector) FabricHealth(c, st string) (json.RawMessage, error) {
	s.note("fabric-health:%s:%s", c, st)
	return json.RawMessage(`{}`), s.err()
}
func (s *stubConnector) FabricTargets(c, st string) (json.RawMessage, error) {
	s.note("fabric-targets:%s:%s", c, st)
	return json.RawMessage(`{}`), s.err()
}
func (s *stubConnector) Invoke(c, st, fn, p string, payload []byte) (json.RawMessage, error) {
	s.note("invoke:%s:%s:%s:%s:%s", c, st, fn, p, string(payload))
	return json.RawMessage(`{}`), s.err()
}
func (s *stubConnector) Logs(c, st, fn, p, svc string) (json.RawMessage, error) {
	s.note("logs:%s:%s:%s:%s:%s", c, st, fn, p, svc)
	return json.RawMessage(`{}`), s.err()
}
func (s *stubConnector) FunctionMetrics(c, st, p, svc string, all bool) (json.RawMessage, error) {
	s.note("fnmetrics:%s:%s:%s:%s:%t", c, st, p, svc, all)
	return json.RawMessage(`{}`), s.err()
}
func (s *stubConnector) Traces(c, st, p, svc string, all bool) (json.RawMessage, error) {
	s.note("traces:%s:%s:%s:%s:%t", c, st, p, svc, all)
	return json.RawMessage(`{}`), s.err()
}
func (s *stubConnector) Doctor(c, st, p string) (json.RawMessage, error) {
	s.note("doctor:%s:%s:%s", c, st, p)
	return json.RawMessage(`{}`), s.err()
}
func (s *stubConnector) Recover(c, st, mode string, dryRun bool) (json.RawMessage, error) {
	s.note("recover:%s:%s:%s:%t", c, st, mode, dryRun)
	return json.RawMessage(`{}`), s.err()
}
func (s *stubConnector) StateOp(op, c, st string, params map[string]string) (json.RawMessage, error) {
	s.note("state:%s:%s:%s:%s", op, c, st, paramString(params))
	return json.RawMessage(`{}`), s.err()
}
func (s *stubConnector) RouterOp(op, c, st string, params map[string]string) (json.RawMessage, error) {
	s.note("routerop:%s:%s:%s:%s", op, c, st, paramString(params))
	return json.RawMessage(`{}`), s.err()
}
func (s *stubConnector) WorkflowOp(op, c, st string, params map[string]string, payload []byte) (json.RawMessage, error) {
	s.note("workflow:%s:%s:%s:%s:%s", op, c, st, paramString(params), string(payload))
	return json.RawMessage(`{}`), s.err()
}

// paramString renders a params map deterministically for call assertions.
func paramString(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if v != "" {
			keys = append(keys, k+"="+v)
		}
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
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

func TestOpsRoutesReachConnector(t *testing.T) {
	srv, stub := newStubServer()

	cases := []struct {
		target string
		want   string
	}{
		{"/fabric/health?stage=prod", "fabric-health:runfabric.yml:prod"},
		{"/fabric/targets?stage=prod", "fabric-targets:runfabric.yml:prod"},
		{"/invoke?stage=prod&function=hello&provider=aws", "invoke:runfabric.yml:prod:hello:aws:"},
		{"/logs?stage=prod&function=hello&service=api", "logs:runfabric.yml:prod:hello::api"},
		{"/metrics/functions?stage=prod&all=1", "fnmetrics:runfabric.yml:prod:::true"},
		{"/traces?stage=prod&service=api", "traces:runfabric.yml:prod::api:false"},
		{"/doctor?stage=prod&provider=gcp", "doctor:runfabric.yml:prod:gcp"},
		{"/recover?stage=prod&dryRun=1", "recover:runfabric.yml:prod:rollback:true"},
		{"/recover?stage=prod&mode=resume", "recover:runfabric.yml:prod:resume:false"},
		{"/state/list?stage=prod", "state:list:runfabric.yml:prod:"},
		{"/state/backup?stage=prod&out=backups/s.json", "state:backup:runfabric.yml:prod:out=backups/s.json"},
		{"/state/migrate?stage=prod&from=local&to=s3", "state:migrate:runfabric.yml:prod:from=local,to=s3"},
		{"/state/unlock?stage=prod&force=true", "state:unlock:runfabric.yml:prod:force=true"},
		{"/router/simulate?stage=prod&requests=50&down=aws", "routerop:simulate:runfabric.yml:prod:down=aws,requests=50"},
		{"/router/shift?stage=prod&provider=gcp&percent=20&dryRun=1", "routerop:shift:runfabric.yml:prod:dryRun=true,percent=20,provider=gcp"},
		{"/router/restore?stage=prod&latest=1", "routerop:restore:runfabric.yml:prod:latest=true"},
		{"/router/history?stage=prod&window=3", "routerop:history:runfabric.yml:prod:window=3"},
		{"/workflow/status?stage=prod&runId=r1", "workflow:status:runfabric.yml:prod:runId=r1:"},
		{"/workflow/cancel?stage=prod&runId=r1", "workflow:cancel:runfabric.yml:prod:runId=r1:"},
		{"/workflow/replay?stage=prod&runId=r1&step=s2", "workflow:replay:runfabric.yml:prod:runId=r1,step=s2:"},
		{"/workflow/runs?stage=prod&limit=5", "workflow:runs:runfabric.yml:prod:limit=5:"},
	}
	for _, tc := range cases {
		stub.calls = nil
		if rec := do(t, srv, "POST", tc.target); rec.Code != 200 {
			t.Fatalf("%s status %d: %s", tc.target, rec.Code, rec.Body.String())
		}
		if got := strings.Join(stub.calls, "|"); got != tc.want {
			t.Fatalf("%s calls = %q, want %q", tc.target, got, tc.want)
		}
	}
}

func TestWorkflowRunCarriesJSONBody(t *testing.T) {
	srv, stub := newStubServer()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/workflow/run?stage=prod&name=etl", strings.NewReader(`{"a":1}`))
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("workflow/run status %d: %s", rec.Code, rec.Body.String())
	}
	want := `workflow:run:runfabric.yml:prod:name=etl:{"a":1}`
	if got := strings.Join(stub.calls, "|"); got != want {
		t.Fatalf("calls = %q, want %q", got, want)
	}
}

func TestInvokeRequiresFunction(t *testing.T) {
	srv, stub := newStubServer()
	if rec := do(t, srv, "POST", "/invoke?stage=prod"); rec.Code != http.StatusBadRequest {
		t.Fatalf("invoke without function = %d, want 400", rec.Code)
	}
	if len(stub.calls) != 0 {
		t.Fatalf("connector reached without function: %v", stub.calls)
	}
}

func TestStatePathParamsAreWorkspaceConfined(t *testing.T) {
	srv, stub := newStubServer()
	for _, target := range []string{
		"/state/backup?out=../escape.json",
		"/state/restore?file=/etc/passwd",
	} {
		if rec := do(t, srv, "POST", target); rec.Code != http.StatusBadRequest {
			t.Fatalf("%s = %d, want 400", target, rec.Code)
		}
	}
	if len(stub.calls) != 0 {
		t.Fatalf("connector reached with escaping path: %v", stub.calls)
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

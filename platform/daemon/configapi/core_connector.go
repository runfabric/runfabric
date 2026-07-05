package configapi

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/runfabric/runfabric/platform/core/model/config"
	"github.com/runfabric/runfabric/platform/workflow/app"
)

// CoreWorkflowConnector is the daemon->core boundary for config/workflow operations.
// provider selects a providerOverrides key for multi-cloud (same semantics as
// the CLI --provider flag); "" targets the config's default provider.
type CoreWorkflowConnector interface {
	Validate(configPath, stage string) error
	Resolve(configPath, stage string) (*ResolveResponse, error)
	Plan(configPath, stage, provider string) (*PlanResponse, error)
	Deploy(configPath, stage, provider string) (*DeployResponse, error)
	Remove(configPath, stage, provider string) (*RemoveResponse, error)
	Releases(configPath string) (*ReleasesResponse, error)
	ReleaseHistory(configPath, stage string) (*ReleasesResponse, error)
	// FabricDeploy deploys to EVERY fabric.targets provider and returns the
	// fabric state (per-provider endpoints).
	FabricDeploy(configPath, stage string) (*FabricDeployResponse, error)
	// RouterSync puts the router over the fabric endpoints (multi-cloud
	// routing config synced through the configured router plugin).
	RouterSync(configPath, stage string, dryRun bool) (*RouterSyncResponse, error)
	// FabricHealth probes every recorded fabric endpoint and returns the
	// fabric state with per-endpoint health.
	FabricHealth(configPath, stage string) (json.RawMessage, error)
	// FabricTargets lists the fabric.targets provider keys of the config.
	FabricTargets(configPath, stage string) (json.RawMessage, error)
	// Invoke calls one deployed function (or workflow orchestration target)
	// with the given JSON payload.
	Invoke(configPath, stage, function, provider string, payload []byte) (json.RawMessage, error)
	// Logs returns provider + local logs for one function ("" = all functions).
	Logs(configPath, stage, function, provider, service string) (json.RawMessage, error)
	// FunctionMetrics returns per-function metrics from the provider.
	FunctionMetrics(configPath, stage, provider, service string, all bool) (json.RawMessage, error)
	// Traces returns traces aggregated by service/stage from the provider.
	Traces(configPath, stage, provider, service string, all bool) (json.RawMessage, error)
	// Doctor runs backend + provider readiness checks (best-effort: a failing
	// side is reported as its error string, not a request failure).
	Doctor(configPath, stage, provider string) (json.RawMessage, error)
	// Recover replays/rolls back an unfinished transaction journal.
	// mode: rollback|resume|inspect; dryRun previews without mutating.
	Recover(configPath, stage, mode string, dryRun bool) (json.RawMessage, error)
	// StateOp runs a state-backend operation:
	// list|pull|backup|restore|reconcile|migrate|unlock|lock-steal.
	// params carries op-specific inputs (out, file, from, to, force) —
	// paths are relative to the daemon workspace.
	StateOp(op, configPath, stage string, params map[string]string) (json.RawMessage, error)
	// RouterOp runs a router operation over the recorded fabric state:
	// history|simulate|verify|shift|restore. params carries op-specific
	// inputs (requests, down, window, provider, percent, snapshot, latest, dryRun).
	RouterOp(op, configPath, stage string, params map[string]string) (json.RawMessage, error)
}

type coreWorkflowAdapter struct{}

func (coreWorkflowAdapter) Validate(configPath, stage string) error {
	cfg, err := config.Load(configPath)
	if err == nil {
		cfg, err = config.Resolve(cfg, stage)
	}
	if err == nil {
		err = config.Validate(cfg)
	}
	return err
}

func (coreWorkflowAdapter) Resolve(configPath, stage string) (*ResolveResponse, error) {
	cfg, err := config.Load(configPath)
	if err == nil {
		cfg, err = config.Resolve(cfg, stage)
	}
	if err != nil {
		return nil, err
	}
	payload, err := marshalPayload(cfg)
	if err != nil {
		return nil, err
	}
	return &ResolveResponse{Payload: payload}, nil
}

func (coreWorkflowAdapter) Plan(configPath, stage, provider string) (*PlanResponse, error) {
	res, err := app.Plan(configPath, stage, provider)
	if err != nil {
		return nil, err
	}
	payload, err := marshalPayload(res)
	if err != nil {
		return nil, err
	}
	return &PlanResponse{Payload: payload}, nil
}

func (coreWorkflowAdapter) Deploy(configPath, stage, provider string) (*DeployResponse, error) {
	res, err := app.Deploy(configPath, stage, "", false, false, nil, provider)
	if err != nil {
		return nil, err
	}
	payload, err := marshalPayload(res)
	if err != nil {
		return nil, err
	}
	return &DeployResponse{Payload: payload}, nil
}

func (coreWorkflowAdapter) Remove(configPath, stage, provider string) (*RemoveResponse, error) {
	res, err := app.Remove(configPath, stage, provider)
	if err != nil {
		return nil, err
	}
	payload, err := marshalPayload(res)
	if err != nil {
		return nil, err
	}
	return &RemoveResponse{Payload: payload}, nil
}

func (coreWorkflowAdapter) Releases(configPath string) (*ReleasesResponse, error) {
	res, err := app.Releases(configPath)
	if err != nil {
		return nil, err
	}
	payload, err := marshalPayload(res)
	if err != nil {
		return nil, err
	}
	return &ReleasesResponse{Payload: payload}, nil
}

func (coreWorkflowAdapter) ReleaseHistory(configPath, stage string) (*ReleasesResponse, error) {
	res, err := app.ReleaseHistory(configPath, stage)
	if err != nil {
		return nil, err
	}
	payload, err := marshalPayload(res)
	if err != nil {
		return nil, err
	}
	return &ReleasesResponse{Payload: payload}, nil
}

func marshalPayload(v any) (json.RawMessage, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal connector payload: %w", err)
	}
	return json.RawMessage(b), nil
}

func (coreWorkflowAdapter) FabricDeploy(configPath, stage string) (*FabricDeployResponse, error) {
	fabricState, err := app.RunFabricDeploy(configPath, stage, false, false)
	if err != nil {
		return nil, err
	}
	if fabricState == nil {
		return nil, fmt.Errorf("fabric is not configured: set fabric.targets and providerOverrides in the config")
	}
	payload, err := marshalPayload(fabricState)
	if err != nil {
		return nil, err
	}
	return &FabricDeployResponse{Payload: payload}, nil
}

func (coreWorkflowAdapter) RouterSync(configPath, stage string, dryRun bool) (*RouterSyncResponse, error) {
	result, routing, err := app.RouterSyncFromFabricState(configPath, stage, dryRun, nil)
	if err != nil {
		return nil, err
	}
	payload, err := marshalPayload(map[string]any{"routing": routing, "result": result})
	if err != nil {
		return nil, err
	}
	return &RouterSyncResponse{Payload: payload}, nil
}

func (coreWorkflowAdapter) FabricHealth(configPath, stage string) (json.RawMessage, error) {
	fabricState, err := app.RunFabricHealth(configPath, stage)
	if err != nil {
		return nil, err
	}
	if fabricState == nil {
		return nil, fmt.Errorf("fabric is not configured: set fabric.targets and providerOverrides in the config")
	}
	return marshalPayload(fabricState)
}

func (coreWorkflowAdapter) FabricTargets(configPath, stage string) (json.RawMessage, error) {
	cfg, err := config.Load(configPath)
	if err == nil {
		cfg, err = config.Resolve(cfg, stage)
	}
	if err != nil {
		return nil, err
	}
	return marshalPayload(map[string]any{"targets": app.RunFabricTargets(cfg)})
}

func (coreWorkflowAdapter) Invoke(configPath, stage, function, provider string, payload []byte) (json.RawMessage, error) {
	res, err := app.Invoke(configPath, stage, function, provider, payload)
	if err != nil {
		return nil, err
	}
	return marshalPayload(res)
}

func (coreWorkflowAdapter) Logs(configPath, stage, function, provider, service string) (json.RawMessage, error) {
	res, err := app.Logs(configPath, stage, function, provider, service)
	if err != nil {
		return nil, err
	}
	return marshalPayload(res)
}

func (coreWorkflowAdapter) FunctionMetrics(configPath, stage, provider, service string, all bool) (json.RawMessage, error) {
	res, err := app.Metrics(configPath, stage, provider, all, service)
	if err != nil {
		return nil, err
	}
	return marshalPayload(res)
}

func (coreWorkflowAdapter) Traces(configPath, stage, provider, service string, all bool) (json.RawMessage, error) {
	res, err := app.Traces(configPath, stage, provider, all, service)
	if err != nil {
		return nil, err
	}
	return marshalPayload(res)
}

func (coreWorkflowAdapter) Doctor(configPath, stage, provider string) (json.RawMessage, error) {
	out := map[string]any{}
	if backend, err := app.BackendDoctor(configPath, stage); err != nil {
		out["backend"] = map[string]any{"error": err.Error()}
	} else {
		out["backend"] = backend
	}
	if probe, err := app.ProviderDoctor(configPath, stage, provider, ""); err != nil {
		out["provider"] = map[string]any{"error": err.Error()}
	} else {
		out["provider"] = probe
	}
	return marshalPayload(out)
}

func (coreWorkflowAdapter) Recover(configPath, stage, mode string, dryRun bool) (json.RawMessage, error) {
	var res any
	var err error
	if dryRun {
		res, err = app.RecoverDryRun(configPath, stage)
	} else {
		res, err = app.RecoverByMode(configPath, stage, mode)
	}
	if err != nil {
		return nil, err
	}
	return marshalPayload(res)
}

func (coreWorkflowAdapter) StateOp(op, configPath, stage string, params map[string]string) (json.RawMessage, error) {
	var res any
	var err error
	switch op {
	case "list":
		res, err = app.StateList(configPath, stage)
	case "pull":
		res, err = app.StatePull(configPath, stage)
	case "backup":
		res, err = app.StateBackup(configPath, stage, params["out"])
	case "restore":
		res, err = app.StateRestore(configPath, stage, params["file"])
	case "reconcile":
		res, err = app.StateReconcile(configPath, stage)
	case "migrate":
		res, err = app.StateMigrate(configPath, stage, params["from"], params["to"])
	case "unlock":
		res, err = app.Unlock(configPath, stage, params["force"] == "true")
	case "lock-steal":
		res, err = app.LockSteal(configPath, stage)
	default:
		return nil, fmt.Errorf("unknown state operation %q", op)
	}
	if err != nil {
		return nil, err
	}
	return marshalPayload(res)
}

func (coreWorkflowAdapter) RouterOp(op, configPath, stage string, params map[string]string) (json.RawMessage, error) {
	var res any
	var err error
	switch op {
	case "history":
		res, err = app.RouterHistoryFromFabricState(configPath, stage, atoiOr(params["window"], 5))
	case "simulate":
		res, err = app.RouterSimulateFromFabricState(configPath, stage, atoiOr(params["requests"], 200), splitList(params["down"]))
	case "verify":
		res, err = app.RouterVerifyFailoverFromFabricState(configPath, stage, atoiOr(params["requests"], 200))
	case "shift":
		res, err = app.RouterShiftFromFabricState(configPath, stage, params["provider"], atoiOr(params["percent"], 0), params["dryRun"] == "true", nil)
	case "restore":
		res, err = app.RouterRestoreFromSnapshot(configPath, stage, params["snapshot"], params["latest"] == "true", params["dryRun"] == "true", nil)
	default:
		return nil, fmt.Errorf("unknown router operation %q", op)
	}
	if err != nil {
		return nil, err
	}
	return marshalPayload(res)
}

func atoiOr(s string, def int) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n <= 0 {
		return def
	}
	return n
}

func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

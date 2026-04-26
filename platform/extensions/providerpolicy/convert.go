package providerpolicy

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"

	providers "github.com/runfabric/runfabric/platform/core/contracts/provider"
	routercontracts "github.com/runfabric/runfabric/platform/core/contracts/router"
	runtimecontracts "github.com/runfabric/runfabric/platform/core/contracts/runtime"
	simulatorcontracts "github.com/runfabric/runfabric/platform/core/contracts/simulators"
)

func toPluginMetaList(items any) []routercontracts.PluginMeta {
	v := reflect.ValueOf(items)
	if !v.IsValid() || v.Kind() != reflect.Slice {
		return nil
	}
	out := make([]routercontracts.PluginMeta, 0, v.Len())
	for i := 0; i < v.Len(); i++ {
		item := v.Index(i)
		if item.Kind() == reflect.Pointer && item.IsNil() {
			continue
		}
		if item.Kind() == reflect.Pointer {
			item = item.Elem()
		}
		out = append(out, routercontracts.PluginMeta{
			ID:          readStringField(item, "ID"),
			Name:        readStringField(item, "Name"),
			Version:     readStringField(item, "Version"),
			Description: readStringField(item, "Description"),
		})
	}
	return out
}

type runtimeRegistry struct {
	raw any
	mu  sync.RWMutex
	ext map[string]runtimecontracts.Runtime
}

func newRuntimeRegistry(raw any) RuntimeRegistry {
	return &runtimeRegistry{raw: raw, ext: map[string]runtimecontracts.Runtime{}}
}

func (r *runtimeRegistry) Get(runtime string) (runtimecontracts.Runtime, error) {
	normalized := NormalizeRuntimeID(runtime)
	if isExternalOnlyRuntime(normalized) {
		r.mu.RLock()
		extPlugin, ok := r.ext[normalized]
		r.mu.RUnlock()
		if ok {
			return extPlugin, nil
		}
		return nil, fmt.Errorf("runtime %q is configured external-only", normalized)
	}
	r.mu.RLock()
	extPlugin, ok := r.ext[normalized]
	r.mu.RUnlock()
	if ok {
		return extPlugin, nil
	}
	rawPlugin, err := callMethod(r.raw, "Get", runtime)
	if err != nil {
		return nil, err
	}
	return &runtimePlugin{plugin: rawPlugin}, nil
}

func (r *runtimeRegistry) Register(runtime runtimecontracts.Runtime) error {
	if runtime == nil {
		return fmt.Errorf("runtime plugin is nil")
	}
	id := NormalizeRuntimeID(strings.TrimSpace(runtime.Meta().ID))
	if id == "" {
		return fmt.Errorf("runtime plugin id is required")
	}
	r.mu.Lock()
	r.ext[id] = runtime
	r.mu.Unlock()
	return nil
}

type runtimePlugin struct {
	plugin any
}

func (p *runtimePlugin) Meta() runtimecontracts.Meta {
	rawMeta, err := callMethodSingle(p.plugin, "Meta")
	if err != nil {
		return runtimecontracts.Meta{}
	}
	v := reflectValue(rawMeta)
	return runtimecontracts.Meta{
		ID:          readStringField(v, "ID"),
		Name:        readStringField(v, "Name"),
		Version:     readStringField(v, "Version"),
		Description: readStringField(v, "Description"),
	}
}

func (p *runtimePlugin) Build(ctx context.Context, req runtimecontracts.BuildRequest) (*providers.Artifact, error) {
	raw, err := callMethod(p.plugin, "Build", ctx, map[string]any{
		"Root":         req.Root,
		"FunctionName": req.FunctionName,
		"Function": map[string]any{
			"Handler": req.FunctionConfig.Handler,
			"Runtime": req.FunctionConfig.Runtime,
		},
		"ConfigSignature": req.ConfigSignature,
	})
	if err != nil {
		return nil, err
	}
	return decodeJSON[providers.Artifact](raw)
}

func (p *runtimePlugin) Invoke(ctx context.Context, req runtimecontracts.InvokeRequest) (*runtimecontracts.InvokeResult, error) {
	raw, err := callMethod(p.plugin, "Invoke", ctx, map[string]any{
		"Root":         req.Root,
		"FunctionName": req.FunctionName,
		"Function": map[string]any{
			"Handler": req.FunctionConfig.Handler,
			"Runtime": req.FunctionConfig.Runtime,
		},
		"Payload": req.Payload,
	})
	if err != nil {
		return nil, err
	}
	return decodeJSON[runtimecontracts.InvokeResult](raw)
}

type simulatorRegistry struct {
	raw any
	mu  sync.RWMutex
	ext map[string]simulatorcontracts.Simulator
}

func newSimulatorRegistry(raw any) SimulatorRegistry {
	return &simulatorRegistry{raw: raw, ext: map[string]simulatorcontracts.Simulator{}}
}

func (r *simulatorRegistry) Get(simulatorID string) (simulatorcontracts.Simulator, error) {
	id := strings.TrimSpace(simulatorID)
	if isExternalOnlySimulator(id) {
		r.mu.RLock()
		extPlugin, ok := r.ext[id]
		r.mu.RUnlock()
		if ok {
			return extPlugin, nil
		}
		return nil, fmt.Errorf("simulator %q is configured external-only", id)
	}
	r.mu.RLock()
	extPlugin, ok := r.ext[id]
	r.mu.RUnlock()
	if ok {
		return extPlugin, nil
	}
	rawPlugin, err := callMethod(r.raw, "Get", simulatorID)
	if err != nil {
		return nil, err
	}
	return &simulatorPlugin{plugin: rawPlugin}, nil
}

func (r *simulatorRegistry) Register(simulator simulatorcontracts.Simulator) error {
	if simulator == nil {
		return fmt.Errorf("simulator plugin is nil")
	}
	id := strings.TrimSpace(simulator.Meta().ID)
	if id == "" {
		return fmt.Errorf("simulator plugin id is required")
	}
	r.mu.Lock()
	r.ext[id] = simulator
	r.mu.Unlock()
	return nil
}

type simulatorPlugin struct {
	plugin any
}

func (p *simulatorPlugin) Meta() simulatorcontracts.Meta {
	rawMeta, err := callMethodSingle(p.plugin, "Meta")
	if err != nil {
		return simulatorcontracts.Meta{}
	}
	v := reflectValue(rawMeta)
	return simulatorcontracts.Meta{
		ID:          readStringField(v, "ID"),
		Name:        readStringField(v, "Name"),
		Description: readStringField(v, "Description"),
	}
}

func (p *simulatorPlugin) Simulate(ctx context.Context, req simulatorcontracts.Request) (*simulatorcontracts.Response, error) {
	raw, err := callMethod(p.plugin, "Simulate", ctx, req)
	if err != nil {
		return nil, err
	}
	return decodeJSON[simulatorcontracts.Response](raw)
}

type routerRegistry struct {
	raw any
	mu  sync.RWMutex
	ext map[string]routercontracts.Router
}

func newRouterRegistry(raw any) RouterRegistry {
	return &routerRegistry{raw: raw, ext: map[string]routercontracts.Router{}}
}

func (r *routerRegistry) Get(routerID string) (routercontracts.Router, error) {
	id := strings.ToLower(strings.TrimSpace(routerID))
	if id == "" {
		return nil, fmt.Errorf("router plugin id is required")
	}
	if isExternalOnlyRouter(id) {
		r.mu.RLock()
		extPlugin, ok := r.ext[id]
		r.mu.RUnlock()
		if ok {
			return extPlugin, nil
		}
		return nil, fmt.Errorf("router %q is configured external-only", id)
	}
	r.mu.RLock()
	extPlugin, ok := r.ext[id]
	r.mu.RUnlock()
	if ok {
		return extPlugin, nil
	}
	rawPlugin, err := callMethod(r.raw, "Get", id)
	if err != nil {
		return nil, err
	}
	return &routerPlugin{plugin: rawPlugin}, nil
}

func (r *routerRegistry) Register(router routercontracts.Router) error {
	if router == nil {
		return fmt.Errorf("router plugin is nil")
	}
	id := strings.ToLower(strings.TrimSpace(router.Meta().ID))
	if id == "" {
		return fmt.Errorf("router plugin id is required")
	}
	r.mu.Lock()
	r.ext[id] = router
	r.mu.Unlock()
	return nil
}

type routerPlugin struct {
	plugin any
}

func (p *routerPlugin) Meta() routercontracts.PluginMeta {
	rawMeta, err := callMethodSingle(p.plugin, "Meta")
	if err != nil {
		return routercontracts.PluginMeta{}
	}
	v := reflectValue(rawMeta)
	return routercontracts.PluginMeta{
		ID:          readStringField(v, "ID"),
		Name:        readStringField(v, "Name"),
		Version:     readStringField(v, "Version"),
		Description: readStringField(v, "Description"),
	}
}

func (p *routerPlugin) Sync(ctx context.Context, req routercontracts.SyncRequest) (*routercontracts.SyncResult, error) {
	raw, err := callRouterSync(p.plugin, ctx, req)
	if err != nil {
		return nil, err
	}
	return decodeJSON[routercontracts.SyncResult](raw)
}

func callRouterSync(plugin any, ctx context.Context, req routercontracts.SyncRequest) (any, error) {
	v := reflect.ValueOf(plugin)
	m := v.MethodByName("Sync")
	if !m.IsValid() {
		return nil, fmt.Errorf("router plugin does not expose Sync")
	}
	mt := m.Type()
	if mt.NumIn() != 2 {
		return nil, fmt.Errorf("router Sync signature mismatch")
	}
	ctxArg, err := convertArg(ctx, mt.In(0))
	if err != nil {
		return nil, err
	}
	syncArg, err := buildRouterSyncArg(req, mt.In(1))
	if err != nil {
		return nil, err
	}
	return invokePair(m, []reflect.Value{ctxArg, syncArg})
}

func buildRouterSyncArg(req routercontracts.SyncRequest, argType reflect.Type) (reflect.Value, error) {
	val := reflect.New(argType).Elem()
	if field := val.FieldByName("ZoneID"); field.IsValid() && field.CanSet() {
		field.SetString(req.ZoneID)
	}
	if field := val.FieldByName("AccountID"); field.IsValid() && field.CanSet() {
		field.SetString(req.AccountID)
	}
	if field := val.FieldByName("DryRun"); field.IsValid() && field.CanSet() {
		field.SetBool(req.DryRun)
	}
	if field := val.FieldByName("Out"); field.IsValid() && field.CanSet() && req.Out != nil {
		outVal := reflect.ValueOf(req.Out)
		if outVal.Type().AssignableTo(field.Type()) {
			field.Set(outVal)
		}
	}

	routingField := val.FieldByName("Routing")
	if !routingField.IsValid() || !routingField.CanSet() || req.Routing == nil {
		return val, nil
	}
	if routingField.Kind() != reflect.Pointer {
		return reflect.Value{}, fmt.Errorf("router sync request type mismatch: Routing is not a pointer")
	}
	routingVal := reflect.New(routingField.Type().Elem()).Elem()
	setStringField(routingVal, "Contract", req.Routing.Contract)
	setStringField(routingVal, "Service", req.Routing.Service)
	setStringField(routingVal, "Stage", req.Routing.Stage)
	setStringField(routingVal, "Hostname", req.Routing.Hostname)
	setStringField(routingVal, "Strategy", req.Routing.Strategy)
	setStringField(routingVal, "HealthPath", req.Routing.HealthPath)
	setIntField(routingVal, "TTL", req.Routing.TTL)

	if endpointsField := routingVal.FieldByName("Endpoints"); endpointsField.IsValid() && endpointsField.CanSet() && endpointsField.Kind() == reflect.Slice {
		elemType := endpointsField.Type().Elem()
		endpoints := reflect.MakeSlice(endpointsField.Type(), 0, len(req.Routing.Endpoints))
		for _, endpoint := range req.Routing.Endpoints {
			ev := reflect.New(elemType).Elem()
			setStringField(ev, "Name", endpoint.Name)
			setStringField(ev, "URL", endpoint.URL)
			if healthyField := ev.FieldByName("Healthy"); healthyField.IsValid() && healthyField.CanSet() && endpoint.Healthy != nil {
				ptr := reflect.New(healthyField.Type().Elem())
				ptr.Elem().SetBool(*endpoint.Healthy)
				healthyField.Set(ptr)
			}
			setIntField(ev, "Weight", endpoint.Weight)
			endpoints = reflect.Append(endpoints, ev)
		}
		endpointsField.Set(endpoints)
	}

	routingField.Set(routingVal.Addr())
	return val, nil
}

func setStringField(v reflect.Value, name, value string) {
	field := v.FieldByName(name)
	if field.IsValid() && field.CanSet() && field.Kind() == reflect.String {
		field.SetString(value)
	}
}

func setIntField(v reflect.Value, name string, value int) {
	field := v.FieldByName(name)
	if field.IsValid() && field.CanSet() && field.Kind() == reflect.Int {
		field.SetInt(int64(value))
	}
}

func adaptDeploy(fn any) func(context.Context, providers.Config, string, string) (*providers.DeployResult, error) {
	return func(ctx context.Context, cfg providers.Config, stage, root string) (*providers.DeployResult, error) {
		raw, err := callFunc(fn, ctx, cfg, stage, root)
		if err != nil {
			return nil, err
		}
		return decodeJSON[providers.DeployResult](raw)
	}
}

func adaptRemove(fn any) func(context.Context, providers.Config, string, string, any) (*providers.RemoveResult, error) {
	return func(ctx context.Context, cfg providers.Config, stage, root string, receipt any) (*providers.RemoveResult, error) {
		raw, err := callFunc(fn, ctx, cfg, stage, root, receipt)
		if err != nil {
			return nil, err
		}
		return decodeJSON[providers.RemoveResult](raw)
	}
}

func adaptInvoke(fn any) func(context.Context, providers.Config, string, string, []byte, any) (*providers.InvokeResult, error) {
	return func(ctx context.Context, cfg providers.Config, stage, function string, payload []byte, receipt any) (*providers.InvokeResult, error) {
		raw, err := callFunc(fn, ctx, cfg, stage, function, payload, receipt)
		if err != nil {
			return nil, err
		}
		return decodeJSON[providers.InvokeResult](raw)
	}
}

func adaptLogs(fn any) func(context.Context, providers.Config, string, string, any) (*providers.LogsResult, error) {
	return func(ctx context.Context, cfg providers.Config, stage, function string, receipt any) (*providers.LogsResult, error) {
		raw, err := callFunc(fn, ctx, cfg, stage, function, receipt)
		if err != nil {
			return nil, err
		}
		return decodeJSON[providers.LogsResult](raw)
	}
}

func adaptPrepareDevStream(fn any) func(context.Context, providers.Config, string, string) (*providers.DevStreamSession, error) {
	return func(ctx context.Context, cfg providers.Config, stage, tunnelURL string) (*providers.DevStreamSession, error) {
		raw, err := callFunc(fn, ctx, cfg, stage, tunnelURL)
		if err != nil || raw == nil {
			return nil, err
		}
		sessionVal := reflect.ValueOf(raw)
		if sessionVal.Kind() == reflect.Pointer && sessionVal.IsNil() {
			return nil, nil
		}
		mode := readStringField(reflectValue(raw), "EffectiveMode")
		message := readStringField(reflectValue(raw), "StatusMessage")
		missing := readStringSliceField(reflectValue(raw), "MissingPrereqs")

		var restore func(context.Context) error
		restoreMethod := sessionVal.MethodByName("Restore")
		if restoreMethod.IsValid() {
			restore = func(c context.Context) error {
				out, invokeErr := invokeSingle(restoreMethod, []reflect.Value{reflect.ValueOf(c)})
				if invokeErr != nil {
					return invokeErr
				}
				if errValue, ok := out.(error); ok {
					return errValue
				}
				return nil
			}
		}
		return providers.NewDevStreamSession(mode, missing, message, restore), nil
	}
}

func adaptFetchMetrics(fn any) func(context.Context, providers.Config, string) (*providers.MetricsResult, error) {
	return func(ctx context.Context, cfg providers.Config, stage string) (*providers.MetricsResult, error) {
		raw, err := callFunc(fn, ctx, cfg, stage)
		if err != nil {
			return nil, err
		}
		return decodeJSON[providers.MetricsResult](raw)
	}
}

func adaptFetchTraces(fn any) func(context.Context, providers.Config, string) (*providers.TracesResult, error) {
	return func(ctx context.Context, cfg providers.Config, stage string) (*providers.TracesResult, error) {
		raw, err := callFunc(fn, ctx, cfg, stage)
		if err != nil {
			return nil, err
		}
		return decodeJSON[providers.TracesResult](raw)
	}
}

func adaptRecover(fn any) func(context.Context, providers.RecoveryRequest) (*providers.RecoveryResult, error) {
	return func(ctx context.Context, req providers.RecoveryRequest) (*providers.RecoveryResult, error) {
		raw, err := callFunc(fn, ctx, req)
		if err != nil {
			return nil, err
		}
		return decodeJSON[providers.RecoveryResult](raw)
	}
}

func adaptSyncOrchestrations(fn any) func(context.Context, providers.OrchestrationSyncRequest) (*providers.OrchestrationSyncResult, error) {
	return func(ctx context.Context, req providers.OrchestrationSyncRequest) (*providers.OrchestrationSyncResult, error) {
		raw, err := callFunc(fn, ctx, req)
		if err != nil {
			return nil, err
		}
		return decodeJSON[providers.OrchestrationSyncResult](raw)
	}
}

func adaptRemoveOrchestrations(fn any) func(context.Context, providers.OrchestrationRemoveRequest) (*providers.OrchestrationSyncResult, error) {
	return func(ctx context.Context, req providers.OrchestrationRemoveRequest) (*providers.OrchestrationSyncResult, error) {
		raw, err := callFunc(fn, ctx, req)
		if err != nil {
			return nil, err
		}
		return decodeJSON[providers.OrchestrationSyncResult](raw)
	}
}

func adaptInvokeOrchestration(fn any) func(context.Context, providers.OrchestrationInvokeRequest) (*providers.InvokeResult, error) {
	return func(ctx context.Context, req providers.OrchestrationInvokeRequest) (*providers.InvokeResult, error) {
		raw, err := callFunc(fn, ctx, req)
		if err != nil {
			return nil, err
		}
		return decodeJSON[providers.InvokeResult](raw)
	}
}

func adaptInspectOrchestrations(fn any) func(context.Context, providers.OrchestrationInspectRequest) (map[string]any, error) {
	return func(ctx context.Context, req providers.OrchestrationInspectRequest) (map[string]any, error) {
		raw, err := callFunc(fn, ctx, req)
		if err != nil {
			return nil, err
		}
		if raw == nil {
			return map[string]any{}, nil
		}
		switch v := raw.(type) {
		case map[string]any:
			return v, nil
		default:
			out := map[string]any{}
			body, marshalErr := json.Marshal(raw)
			if marshalErr != nil {
				return nil, marshalErr
			}
			if unmarshalErr := json.Unmarshal(body, &out); unmarshalErr != nil {
				return nil, unmarshalErr
			}
			return out, nil
		}
	}
}

func callFunc(fn any, args ...any) (any, error) {
	return invokePair(reflect.ValueOf(fn), toArgValues(reflect.ValueOf(fn), args))
}

func callMethod(target any, method string, args ...any) (any, error) {
	v := reflect.ValueOf(target)
	if !v.IsValid() {
		return nil, fmt.Errorf("invalid target for %s", method)
	}
	m := v.MethodByName(method)
	if !m.IsValid() {
		return nil, fmt.Errorf("method %s not found", method)
	}
	return invokePair(m, toArgValues(m, args))
}

func callMethodSingle(target any, method string, args ...any) (any, error) {
	v := reflect.ValueOf(target)
	if !v.IsValid() {
		return nil, fmt.Errorf("invalid target for %s", method)
	}
	m := v.MethodByName(method)
	if !m.IsValid() {
		return nil, fmt.Errorf("method %s not found", method)
	}
	return invokeSingle(m, toArgValues(m, args))
}

func toArgValues(callable reflect.Value, args []any) []reflect.Value {
	mt := callable.Type()
	in := make([]reflect.Value, 0, len(args))
	for i, arg := range args {
		if i >= mt.NumIn() {
			break
		}
		value, err := convertArg(arg, mt.In(i))
		if err != nil {
			in = append(in, reflect.Zero(mt.In(i)))
			continue
		}
		in = append(in, value)
	}
	return in
}

func invokePair(callable reflect.Value, in []reflect.Value) (any, error) {
	if !callable.IsValid() || callable.Kind() != reflect.Func {
		return nil, fmt.Errorf("invalid callable")
	}
	mt := callable.Type()
	if len(in) != mt.NumIn() {
		return nil, fmt.Errorf("callable expects %d args, got %d", mt.NumIn(), len(in))
	}
	out := callable.Call(in)
	if len(out) != 2 {
		return nil, fmt.Errorf("callable must return (value, error)")
	}
	if !out[1].IsNil() {
		err, ok := out[1].Interface().(error)
		if ok {
			return nil, err
		}
		return nil, fmt.Errorf("callable returned non-error second value")
	}
	if out[0].Kind() == reflect.Pointer && out[0].IsNil() {
		return nil, nil
	}
	return out[0].Interface(), nil
}

func invokeSingle(callable reflect.Value, in []reflect.Value) (any, error) {
	if !callable.IsValid() || callable.Kind() != reflect.Func {
		return nil, fmt.Errorf("invalid callable")
	}
	mt := callable.Type()
	if len(in) != mt.NumIn() {
		return nil, fmt.Errorf("callable expects %d args, got %d", mt.NumIn(), len(in))
	}
	out := callable.Call(in)
	if len(out) != 1 {
		return nil, fmt.Errorf("callable must return one value")
	}
	if out[0].Kind() == reflect.Pointer && out[0].IsNil() {
		return nil, nil
	}
	return out[0].Interface(), nil
}

func convertArg(arg any, targetType reflect.Type) (reflect.Value, error) {
	if arg == nil {
		return reflect.Zero(targetType), nil
	}
	val := reflect.ValueOf(arg)
	if val.Type().AssignableTo(targetType) {
		return val, nil
	}
	if val.Type().ConvertibleTo(targetType) {
		return val.Convert(targetType), nil
	}
	if targetType.Kind() == reflect.Interface && val.Type().Implements(targetType) {
		return val, nil
	}
	body, err := json.Marshal(arg)
	if err != nil {
		return reflect.Value{}, err
	}
	dst := reflect.New(targetType)
	if err := json.Unmarshal(body, dst.Interface()); err != nil {
		return reflect.Value{}, err
	}
	return dst.Elem(), nil
}

func decodeJSON[T any](raw any) (*T, error) {
	if raw == nil {
		return nil, nil
	}
	if direct, ok := raw.(*T); ok {
		return direct, nil
	}
	body, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var out T
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func reflectValue(v any) reflect.Value {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer && !rv.IsNil() {
		return rv.Elem()
	}
	return rv
}

func readStringField(v reflect.Value, name string) string {
	if !v.IsValid() {
		return ""
	}
	field := v.FieldByName(name)
	if !field.IsValid() || field.Kind() != reflect.String {
		return ""
	}
	return field.String()
}

func readStringSliceField(v reflect.Value, name string) []string {
	if !v.IsValid() {
		return nil
	}
	field := v.FieldByName(name)
	if !field.IsValid() || field.Kind() != reflect.Slice {
		return nil
	}
	out := make([]string, 0, field.Len())
	for i := 0; i < field.Len(); i++ {
		elem := field.Index(i)
		if elem.Kind() == reflect.String {
			out = append(out, elem.String())
		}
	}
	return out
}

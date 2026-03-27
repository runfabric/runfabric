# RunFabric Go Plugin SDK

Minimal SDK for building external RunFabric plugins without importing the engine module.

## Includes

- JSON wire types (`protocol.Request`, `protocol.Response`)
- Stdio server loop (`server.Server`) with method dispatch
- Consistent response envelope with request IDs and error fields
- Typed provider contract (`provider.Plugin`) with typed request/response models
- Provider adapter (`provider.NewServer`) to expose typed plugins over the wire protocol
- Optional typed capabilities: `provider.ObservabilityCapable`, `provider.DevStreamCapable`, `provider.RecoveryCapable`, `provider.OrchestrationCapable`

## Example

```go
package main

import (
  "context"
  "encoding/json"
  "os"

  "github.com/runfabric/runfabric/plugin-sdk/go/server"
)

func main() {
  s := server.New(server.Options{
    ProtocolVersion: "1",
    Handshake: server.HandshakeMetadata{
      Version: "0.1.0",
      Capabilities: []string{"doctor"},
      SupportsRuntime: []string{"nodejs"},
      SupportsTriggers: []string{"http"},
    },
    Methods: map[string]server.MethodFunc{
      "Doctor": func(ctx context.Context, params json.RawMessage) (any, error) {
        return map[string]any{"checks": []string{"ok"}}, nil
      },
    },
  })
  _ = s.Serve(context.Background(), os.Stdin, os.Stdout)
}
```

Run tests:

```bash
cd packages/go/plugin-sdk
go test ./...
```

## Typed provider API

Use `provider.Plugin` when you want compile-time request/response contracts without importing engine internals:

```go
package main

import (
  "context"
  "os"

  sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

type plugin struct{}

func (p *plugin) Meta() sdkprovider.Meta {
  return sdkprovider.Meta{
    Name: "acme-provider",
    Version: "0.1.0",
    SupportsRuntime: []string{"nodejs"},
    SupportsTriggers: []string{"http"},
  }
}

func (p *plugin) ValidateConfig(ctx context.Context, req sdkprovider.ValidateConfigRequest) error { return nil }
func (p *plugin) Doctor(ctx context.Context, req sdkprovider.DoctorRequest) (*sdkprovider.DoctorResult, error) {
  return &sdkprovider.DoctorResult{Provider: "acme-provider", Checks: []string{"ok"}}, nil
}
func (p *plugin) Plan(ctx context.Context, req sdkprovider.PlanRequest) (*sdkprovider.PlanResult, error) {
  return &sdkprovider.PlanResult{Provider: "acme-provider"}, nil
}
func (p *plugin) Deploy(ctx context.Context, req sdkprovider.DeployRequest) (*sdkprovider.DeployResult, error) {
  return &sdkprovider.DeployResult{Provider: "acme-provider", DeploymentID: "deploy-1"}, nil
}
func (p *plugin) Remove(ctx context.Context, req sdkprovider.RemoveRequest) (*sdkprovider.RemoveResult, error) {
  return &sdkprovider.RemoveResult{Provider: "acme-provider", Removed: true}, nil
}
func (p *plugin) Invoke(ctx context.Context, req sdkprovider.InvokeRequest) (*sdkprovider.InvokeResult, error) {
  return &sdkprovider.InvokeResult{Provider: "acme-provider"}, nil
}
func (p *plugin) Logs(ctx context.Context, req sdkprovider.LogsRequest) (*sdkprovider.LogsResult, error) {
  return &sdkprovider.LogsResult{Provider: "acme-provider"}, nil
}

func main() {
  s := sdkprovider.NewServer(&plugin{}, sdkprovider.ServeOptions{ProtocolVersion: "1"})
  _ = s.Serve(context.Background(), os.Stdin, os.Stdout)
}
```

## Migration guide: raw handlers -> typed plugin

Before (raw method map):

```go
package main

import (
  "context"
  "encoding/json"
  "os"

  "github.com/runfabric/runfabric/plugin-sdk/go/server"
)

func main() {
  s := server.New(server.Options{
    ProtocolVersion: "1",
    Handshake: server.HandshakeMetadata{
      Version: "0.1.0",
      Capabilities: []string{"doctor", "plan", "deploy", "remove", "invoke", "logs"},
      SupportsRuntime: []string{"nodejs"},
      SupportsTriggers: []string{"http"},
    },
    Methods: map[string]server.MethodFunc{
      "Doctor": func(ctx context.Context, params json.RawMessage) (any, error) {
        return map[string]any{"provider": "acme-provider", "checks": []string{"ok"}}, nil
      },
      "Deploy": func(ctx context.Context, params json.RawMessage) (any, error) {
        return map[string]any{"provider": "acme-provider", "deploymentId": "dep-1"}, nil
      },
    },
  })
  _ = s.Serve(context.Background(), os.Stdin, os.Stdout)
}
```

After (typed plugin):

```go
package main

import (
  "context"
  "os"

  sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

type plugin struct{}

func (p *plugin) Meta() sdkprovider.Meta {
  return sdkprovider.Meta{
    Name: "acme-provider",
    Version: "0.1.0",
    SupportsRuntime: []string{"nodejs"},
    SupportsTriggers: []string{"http"},
  }
}

func (p *plugin) ValidateConfig(ctx context.Context, req sdkprovider.ValidateConfigRequest) error { return nil }
func (p *plugin) Doctor(ctx context.Context, req sdkprovider.DoctorRequest) (*sdkprovider.DoctorResult, error) {
  return &sdkprovider.DoctorResult{Provider: "acme-provider", Checks: []string{"ok"}}, nil
}
func (p *plugin) Plan(ctx context.Context, req sdkprovider.PlanRequest) (*sdkprovider.PlanResult, error) {
  return &sdkprovider.PlanResult{Provider: "acme-provider"}, nil
}
func (p *plugin) Deploy(ctx context.Context, req sdkprovider.DeployRequest) (*sdkprovider.DeployResult, error) {
  return &sdkprovider.DeployResult{Provider: "acme-provider", DeploymentID: "dep-1"}, nil
}
func (p *plugin) Remove(ctx context.Context, req sdkprovider.RemoveRequest) (*sdkprovider.RemoveResult, error) {
  return &sdkprovider.RemoveResult{Provider: "acme-provider", Removed: true}, nil
}
func (p *plugin) Invoke(ctx context.Context, req sdkprovider.InvokeRequest) (*sdkprovider.InvokeResult, error) {
  return &sdkprovider.InvokeResult{Provider: "acme-provider"}, nil
}
func (p *plugin) Logs(ctx context.Context, req sdkprovider.LogsRequest) (*sdkprovider.LogsResult, error) {
  return &sdkprovider.LogsResult{Provider: "acme-provider"}, nil
}

func main() {
  s := sdkprovider.NewServer(&plugin{}, sdkprovider.ServeOptions{ProtocolVersion: "1"})
  _ = s.Serve(context.Background(), os.Stdin, os.Stdout)
}
```

Optional typed capabilities are discovered automatically by `provider.NewServer`:

- implement `provider.ObservabilityCapable` -> binds `FetchMetrics` and `FetchTraces`, advertises `observability`
- implement `provider.DevStreamCapable` -> binds `PrepareDevStream`, advertises `dev-stream`
- implement `provider.RecoveryCapable` -> binds `Recover`, advertises `recovery`
- implement `provider.OrchestrationCapable` -> binds `SyncOrchestrations`, `RemoveOrchestrations`, `InvokeOrchestration`, `InspectOrchestrations`, advertises `orchestration`

### Small typed orchestration example

```go
type plugin struct{}

func (p *plugin) SyncOrchestrations(ctx context.Context, req sdkprovider.OrchestrationSyncRequest) (*sdkprovider.OrchestrationSyncResult, error) {
  return &sdkprovider.OrchestrationSyncResult{
    Outputs: map[string]string{"orchestration": "synced"},
  }, nil
}

func (p *plugin) RemoveOrchestrations(ctx context.Context, req sdkprovider.OrchestrationRemoveRequest) (*sdkprovider.OrchestrationSyncResult, error) {
  return &sdkprovider.OrchestrationSyncResult{
    Outputs: map[string]string{"orchestration": "removed"},
  }, nil
}

func (p *plugin) InvokeOrchestration(ctx context.Context, req sdkprovider.OrchestrationInvokeRequest) (*sdkprovider.InvokeResult, error) {
  return &sdkprovider.InvokeResult{
    Provider: "acme-provider",
    Function: "orchestration:" + req.Name,
    Output:   "started",
    Workflow: req.Name,
  }, nil
}

func (p *plugin) InspectOrchestrations(ctx context.Context, req sdkprovider.OrchestrationInspectRequest) (map[string]any, error) {
  return map[string]any{
    "count":  1,
    "status": "ok",
  }, nil
}
```

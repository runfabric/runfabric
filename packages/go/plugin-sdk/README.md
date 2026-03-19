# RunFabric Go Plugin SDK

Minimal SDK for building external RunFabric plugins without importing the engine module.

## Includes

- JSON wire types (`protocol.Request`, `protocol.Response`)
- Stdio server loop (`server.Server`) with method dispatch
- Consistent response envelope with request IDs and error fields

## Example

```go
package main

import (
  "context"
  "encoding/json"
  "os"

  "github.com/runfabric/runfabric/plugin-sdk/go/protocol"
  "github.com/runfabric/runfabric/plugin-sdk/go/server"
)

func main() {
  s := server.New(server.Options{
    ProtocolVersion: "2025-01-01",
    Methods: map[string]server.MethodFunc{
      "handshake": func(ctx context.Context, params json.RawMessage) (any, error) {
        return map[string]any{"name": "example-plugin"}, nil
      },
      "provider.doctor": func(ctx context.Context, params json.RawMessage) (any, error) {
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

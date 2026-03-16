# RunFabric SDK for Go

Handler contract and HTTP adapter for RunFabric functions in Go.

## Handler contract

```go
import "github.com/runfabric/runfabric/sdk/go/handler"

h := handler.Func(func(event map[string]any, runCtx *handler.Context) map[string]any {
    return map[string]any{"message": "hello", "stage": runCtx.Stage}
})
```

## HTTP adapter

Use `HTTPHandler` to wrap your handler for `net/http` or any HTTP framework:

```go
http.ListenAndServe(":3000", handler.HTTPHandler(h))
```

## Build and test

```bash
go build ./...
go test ./...
```

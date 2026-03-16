# RunFabric SDK for .NET

Handler contract and HTTP adapter for RunFabric functions in C# / .NET.

## Handler contract

```csharp
using RunFabric.Sdk;

Handler h = (event, context) => new Dictionary<string, object?>
{
    ["message"] = "hello",
    ["stage"] = context.Stage
};
```

## HTTP adapter

Use `HttpHandler.InvokeAsync` to process a request stream and write the response:

```csharp
await HttpHandler.InvokeAsync(requestBody, responseBody, h, stage: "dev", functionName: "api", requestId: "...");
```

## Build and test

```bash
dotnet build
dotnet test
```

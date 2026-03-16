# OpenTelemetry

RunFabric can export **traces** via OpenTelemetry (OTLP) when configured. The CLI and daemon initialize the SDK at startup and shut it down on exit. When no exporter is configured, tracing is no-op (no overhead).

## Environment variables

| Variable | Description |
|----------|--------------|
| `OTEL_SERVICE_NAME` | Service name for the resource (default: `runfabric`) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP endpoint as host:port (e.g. `localhost:4318`) or URL; when set, traces are exported over HTTP. |
| `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` | Traces-specific endpoint; overrides `OTEL_EXPORTER_OTLP_ENDPOINT` for traces only |
| `OTEL_TRACES_ENABLED` | Set to `1` or `true` to enable tracing; if no endpoint is set, defaults to `localhost:4318` |

Protocol is **OTLP over HTTP**. Use an OpenTelemetry Collector or a backend (e.g. Jaeger, Honeycomb, Grafana Tempo) that accepts OTLP.

## Enabling tracing

```bash
# Export to a local collector (e.g. otelcol) on default HTTP port
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
runfabric daemon --dashboard --config runfabric.yml

# Or enable with default endpoint
export OTEL_TRACES_ENABLED=1
runfabric daemon
```

## What is traced

- **Daemon:** Each HTTP request gets a span with `http.method` and `http.route`. Spans are created by the `runfabric/daemon` tracer. On 4xx/5xx responses the span status is set to Error.

CLI commands (plan, deploy, invoke, etc.) do not yet create spans; you can add them by calling `telemetry.Tracer("runfabric/<command>")` and starting spans in the relevant packages.

## Shutdown

The CLI calls `telemetry.Shutdown` with a 5-second timeout on exit so pending spans are flushed before the process ends.

## See also

- [DAEMON.md](DAEMON.md) — Daemon options and API
- [OpenTelemetry Go](https://opentelemetry.io/docs/languages/go/) — SDK and exporter docs

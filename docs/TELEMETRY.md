# OpenTelemetry

RunFabric can export **traces** via OpenTelemetry (OTLP) when configured. The CLI and daemon initialize the SDK at startup and shut it down on exit. When no exporter is configured, tracing is no-op (no overhead).

## Quick navigation

- **Which env vars matter**: Environment variables
- **Enable tracing quickly**: Enabling tracing
- **What gets traced**: What is traced

## Environment variables

| Variable | Description |
|----------|--------------|
| `OTEL_SERVICE_NAME` | Service name for the resource (default: `runfabric`) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP endpoint as host:port (e.g. `localhost:4318`) or URL; when set, traces are exported over HTTP. |
| `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` | Traces-specific endpoint; overrides `OTEL_EXPORTER_OTLP_ENDPOINT` for traces only |
| `OTEL_TRACES_ENABLED` | Set to `1` or `true` to enable tracing; if no endpoint is set, defaults to `localhost:4318` |
| `OTEL_TRACES_SAMPLER` | Sampler: `always_on`, `always_off`, `traceidratio`, `parentbased_always_on`, `parentbased_traceidratio` (default: `always_on` when exporter set) |
| `OTEL_TRACES_SAMPLER_ARG` | For `traceidratio` / `parentbased_traceidratio`: sampling ratio 0.0–1.0 (e.g. `0.1` = 10%) to reduce overhead in production |

Protocol is **OTLP over HTTP**. Use an OpenTelemetry Collector or a backend (e.g. Jaeger, Honeycomb, Grafana Tempo) that accepts OTLP.

## Enabling tracing

```bash
# Export to a local collector (e.g. otelcol) on default HTTP port
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
runfabricd --dashboard --config runfabric.yml

# Or enable with default endpoint
export OTEL_TRACES_ENABLED=1
runfabricd
```

## What is traced

- **Daemon:** Each HTTP request gets a span with `http.method` and `http.route`. Spans are created by the `runfabric/daemon` tracer. On 4xx/5xx responses the span status is set to Error.
- **CLI:** The `plan`, `deploy`, and `invoke` commands create a span each (tracers `runfabric/plan`, `runfabric/deploy`, `runfabric/invoke`) with attributes: `config_path`, `stage`, `function_name` (where applicable), `provider_override` (when set).

Key span attributes: `http.method`, `http.route`, `http.status_code` (daemon); `config_path`, `stage`, `function_name`, `provider_override` (CLI).

## Shutdown

The CLI calls `telemetry.Shutdown` with a 5-second timeout on exit so pending spans are flushed before the process ends.

## Privacy

By default, span attributes include config path, stage, and function names—not secrets or request bodies. Do not put secrets (API keys, tokens, connection strings) into custom span attributes or log lines. Use `OTEL_TRACES_SAMPLER_ARG` (e.g. `0.1`) in production to limit volume if needed.

## See also

- [DAEMON.md](DAEMON.md) — Daemon options and API
- [OpenTelemetry Go](https://opentelemetry.io/docs/languages/go/) — SDK and exporter docs

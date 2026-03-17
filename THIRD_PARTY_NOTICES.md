# Third-Party Notices

RunFabric uses the following key dependencies. Licenses are documented here for compliance. See each project for full license text.

## Go (engine)

| Dependency | License | Notes |
|------------|---------|--------|
| [AWS SDK for Go v2](https://github.com/aws/aws-sdk-go-v2) | Apache-2.0 | Lambda, API Gateway, S3, DynamoDB, etc. |
| [Cobra](https://github.com/spf13/cobra) | Apache-2.0 | CLI framework |
| [OpenTelemetry Go](https://github.com/open-telemetry/opentelemetry-go) | Apache-2.0 | Tracing SDK and OTLP exporter |
| [go-redis](https://github.com/redis/go-redis) | BSD-3-Clause | Redis client for daemon API cache |
| [pgx](https://github.com/jackc/pgx) | MIT | Postgres driver for state backends |
| [modernc.org/sqlite](https://modernc.org/sqlite) | BSD-3-Clause | SQLite for local/sqlite state |
| [k8s.io/client-go](https://github.com/kubernetes/client-go) | Apache-2.0 | Kubernetes API client |
| [gopkg.in/yaml.v3](https://github.com/go-yaml/yaml) | Apache-2.0, MIT | YAML parsing |
| [golang.org/x/term](https://pkg.go.dev/golang.org/x/term) | BSD-3-Clause | Terminal handling |

## Node / TypeScript (packages, protocol)

See `packages/node/*/package.json` and `protocol/mcp/package.json` for per-package licenses (e.g. MIT, Apache-2.0).

## Python (packages)

See `packages/python/runfabric/pyproject.toml` and installed package licenses (e.g. MIT).

---

This file is provided for transparency. When distributing RunFabric, ensure you comply with each dependency’s license terms. For the exact versions in use, see `engine/go.sum`, `package-lock.json`, and lockfiles under `packages/`.

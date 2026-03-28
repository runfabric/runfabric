# RunFabric

**RunFabric is a multi-provider serverless framework with a unified config and CLI workflow for services, functions, resources, and workflows.**

- **One config** — Single `runfabric.yml` per service; same structure whether you target AWS, GCP, Azure, Cloudflare, Vercel, Netlify, or others.
- **One CLI workflow** — Same commands everywhere: `runfabric doctor`, `plan`, `build`, `deploy`, `invoke`, `logs`, `remove`.
- **Managed serverless** — Deploys to each provider’s managed offerings (Lambda, Cloud Functions, Workers, etc.), which auto-scale and charge for usage rather than idle capacity.

**Current scope:**

- **Core engine and CLI** — Go code in `cmd/`, `internal/`, and `platform/`; build with `make build` → `bin/runfabric`, `bin/runfabricw`, and `bin/runfabricd`. **Packages** (Node CLI/SDK, Python, Go, Java, .NET) live under `packages/`. See [docs/FILE_STRUCTURE.md](docs/FILE_STRUCTURE.md) and [docs/LAYOUT.md](docs/LAYOUT.md).
- **Deployment framework** — Not a cluster scheduler or standalone runtime; uses each provider’s managed serverless (Lambda, Cloud Run, Workers, etc.).
- **Config** — `runfabric.yml` (not a drop-in replacement for `serverless.yml`; use `runfabric migrate` to convert).
- **Production path** — Node-first beta; other runtimes (Python, Go) are supported per provider.

## Why RunFabric

- **One config, one CLI** — One `runfabric.yml` and the same `runfabric` commands across all supported providers.
- **Portability** — Provider and trigger checks before deploy; switch clouds without rewriting your app.
- **Validation** — `runfabric doctor` checks config and credentials.
- **Alternative** — Use instead of or alongside [Serverless Framework](https://github.com/serverless/serverless); migrate with `runfabric migrate`.

## Features

- Unified lifecycle: `doctor -> plan -> build -> deploy -> remove`
- Interactive project scaffold: `runfabric init`
- Local provider-mimic execution: `runfabric call-local`
- Local dev loop with rebuild + presets: `runfabric dev`
- Multi-provider invoke/logs/remove support
- Trace and metric queries from local artifacts: `runfabric traces`, `runfabric metrics`
- Compose orchestration (`plan|deploy|remove`) with dependency ordering and bounded concurrency
- Stage-aware local state and deployment receipts
- Dynamic env interpolation in config values (`${env:VAR}` and `${env:VAR,default}`)
- Node-first runtime guardrails for current beta (`runtime: nodejs` production-ready path)
- Extended trigger model: `http`, `cron`, `queue`, `storage`, `eventbridge`, `pubsub`, `kafka`, `rabbitmq`
- Workflow/resources/secrets schema support in `runfabric.yml`

## Install CLI

**Option 1 — Build from source (recommended for development):**

```bash
git clone <repo>
cd runfaric
make build
./bin/runfabric --help
```

**Option 2 — npm (Node wrapper around the engine binary):**

```bash
npm install -g @runfabric/cli
runfabric --help
```

Or without global install: `npx @runfabric/cli@latest --help`.

The npm package bundles or fetches the RunFabric binary for your OS; the core is the Go engine in this repo (`cmd/`, `internal/`, `platform/`). The repo uses a **Makefile** for build, test, and release (no root `package.json`). See `CONTRIBUTING.md` and `make help`.

**Apple Silicon (arm64):** Use the native binary from `make build`. If the binary is **killed**, it may be quarantined (e.g. after copying from CI); run `make bin-clear-quarantine` or `xattr -cr bin/` then try again.

If the npm package is not yet published, build from source and use `./bin/runfabric` or follow `docs/REPO_DEVELOPMENT.md`.

## Quick Start

Create a new project:

```bash
runfabric init --dir ./my-api
```

Interactive `init` uses grouped pickers with type-to-filter search (`Up/Down`, `Enter`, `Esc` to clear filter).
Template choices are provider-aware (unsupported templates are hidden or rejected in non-interactive mode).

Default service name is derived from the target directory (`my-api` here). Override with `--service` when needed.

Explicit state backend selection:

```bash
runfabric init --dir ./my-api --provider aws-lambda --state-backend s3
```

`init` now generates `.env.example` with provider + selected state backend variables.
For object-storage backends (`s3`, `gcs`, `azblob`), `init` also generates a project-scoped random state prefix to avoid collisions.
Copy and load it before deploy:

```bash
cd my-api
cp .env.example .env
set -a
source .env
set +a
```

Migrate an existing Serverless Framework config (best-effort bootstrap):

```bash
runfabric migrate --input ./serverless.yml --output ./runfabric.yml --json
```

Set provider credentials (provider-specific list):

- `docs/CREDENTIALS.md`
- `docs/PROVIDER_SETUP.md`

Run lifecycle:

```bash
runfabric doctor -c ./my-api/runfabric.yml
runfabric plan -c ./my-api/runfabric.yml
runfabric build -c ./my-api/runfabric.yml
runfabric deploy -c ./my-api/runfabric.yml
runfabric deploy -c ./my-api/runfabric.yml --rollback-on-failure
```

Workflow runtime (workload-plane binary):

```bash
./bin/runfabricw workflow run -c ./my-api/runfabric.yml --name hello-flow
./bin/runfabricw workflow status -c ./my-api/runfabric.yml --run-id <run-id>
```

Run local provider-mimic server:

```bash
cd my-api
npm run call:local
curl -i http://127.0.0.1:8787/hello
# stop server: Ctrl+C or type 'exit' and press Enter
npm run call:local -- --serve --event ./event.template.json
```

`call:local` now runs in watch mode by default in scaffolded projects.

## Framework wrappers (Express, Fastify, Nest)

The **Node SDK** (`@runfabric/sdk`, `packages/node/sdk`) lets you use a single handler or mount RunFabric handlers in Express, Fastify, or Nest. The **Node CLI** is `@runfabric/cli` (`packages/node/cli`). See `packages/node/sdk/README.md` for `createHandler`, `mountExpress`, and `mountFastify`.

## Core Commands

- `runfabric init`
- `runfabric docs check|sync`
- `runfabric doctor`
- `runfabric plan`
- `runfabric build`
- `runfabric package`
- `runfabric deploy` (optional: `--function <name>`, `--rollback-on-failure` / `--no-rollback-on-failure`, `--out <dir>`)
- `runfabric deploy fn <name>` | `runfabric deploy function <name>` | `runfabric deploy-function <name>` (single-function deploy)
- `runfabric call-local`
- `runfabric dev`
- `runfabric migrate`
- `runfabric invoke`
- `runfabric logs`
- `runfabric traces`
- `runfabric metrics`
- `runfabric remove`
- `runfabric router deploy|status|endpoints|routing|simulate|chaos-verify|dns-sync|dns-shift|dns-reconcile|dns-restore|dns-history`
- `runfabric compose plan|deploy|remove`
- `runfabric state pull|list|backup|restore|force-unlock|migrate|reconcile`

Rollback precedence: CLI `--rollback-on-failure` / `--no-rollback-on-failure` → `runfabric.yml` `deploy.rollbackOnFailure` → `RUNFABRIC_ROLLBACK_ON_FAILURE`. Full command reference: [docs/COMMAND_REFERENCE.md](docs/COMMAND_REFERENCE.md).

## Security scanning

- **CI:** Snyk workflow in `.github/workflows/snyk.yml` (requires `SNYK_TOKEN` for full scan).
- Run `invoke` and `logs` from the project root so the CLI can resolve config and context.

## Supported providers

- `aws-lambda`
- `gcp-functions`
- `azure-functions`
- `kubernetes`
- `cloudflare-workers`
- `vercel`
- `netlify`
- `alibaba-fc`
- `digitalocean-functions`
- `fly-machines`
- `ibm-openwhisk`

## State And Receipts

- Deploy receipt: `.runfabric/deploy/<provider>/deployment.json`
- Local backend state file: `.runfabric/state/<service>/<stage>/<provider>.state.json`
- Remote backends (`postgres|s3|gcs|azblob`) store state in the configured external backend.

## Documentation

- `docs/QUICKSTART.md` — get started
- `docs/COMMAND_REFERENCE.md` — CLI commands and flags
- `docs/RUNFABRIC_YML_REFERENCE.md` — runfabric.yml config
- `docs/CREDENTIALS.md` — provider and state backend credentials
- `docs/PROVIDER_SETUP.md` — per-provider setup
- `docs/DEPLOY_PROVIDERS.md` — deploy by provider (REST/SDK)
- `docs/STATE_BACKENDS.md` — state storage
- `docs/ROUTER_OPERATIONS_WORKFLOW.md` — router deploy/sync/verify/rollback runbook
- `docs/ROUTER_PLUGIN_AUTHORING.md` — router plugin contract + Go template

- `docs/MIGRATION.md` — Serverless Framework migration
- `docs/ARCHITECTURE.md` — deploy flow and provider layout
- `docs/BUILD_AND_RELEASE.md` — build and release
- `docs/REPO_DEVELOPMENT.md` — contributor setup
- `apps/registry/docs/PLUGINS.md` — plugins (lifecycle hooks) and API contract
- `docs/EXAMPLES_MATRIX.md` — examples and trigger matrix
- `docs/EXAMPLE_VALIDATION.md` — example checklist
- `docs/README.md` — doc index

## Contributing

- `CONTRIBUTING.md` — setup, quality checks, and **pre-push hook** (`git config core.hooksPath .githooks` to enable lint + validation on push)
- `CODE_OF_CONDUCT.md`
- `AGENTS.md`

## Versioning

See `VERSIONING.md` for version scheme and compatibility.

## License

RunFabric License (`LICENSE`). You may use the Software for your own projects (including commercial projects). You may not offer the Software, or a service based on it, as a cloud, hosted, or managed service to third parties without a separate license from the copyright holders.

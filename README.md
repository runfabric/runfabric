# runfabric

**Alternate of Serverless Framework**

`runfabric` is a multi-provider serverless framework package. It gives you one config and one CLI workflow across cloud providers, so you can deploy on managed serverless services that auto-scale and keep idle-cost overhead low.

Project scope (current):

- CLI/serverless deployment framework, not a standalone scheduler/runtime fabric.
- Uses `runfabric.yml`; not a drop-in config replacement for `serverless.yml`.
- Node-first beta (`runtime: nodejs` production-ready path in current release train).

## Why runfabric

- Single service config: `runfabric.yml`
- Provider portability checks before deploy
- Provider credential schema + `doctor` validation
- Works as an alternative to [Serverless Framework](https://github.com/serverless/serverless)

## Features

- Unified lifecycle: `doctor -> plan -> build -> deploy -> remove`
- Interactive project scaffold: `runfabric init`
- Local provider-mimic execution: `runfabric call-local`
- Local dev loop with rebuild + presets: `runfabric dev`
- Multi-provider invoke/logs/remove support
- Trace and metric queries from local artifacts: `runfabric traces`, `runfabric metrics`
- Compose deploy orchestration with dependency ordering
- Stage-aware local state and deployment receipts
- Dynamic env interpolation in config values (`${env:VAR}` and `${env:VAR,default}`)
- Node-first runtime guardrails for current beta (`runtime: nodejs` production-ready path)
- Extended trigger model: `http`, `cron`, `queue`, `storage`, `eventbridge`, `pubsub`, `kafka`, `rabbitmq`
- Workflow/resources/secrets schema support in `runfabric.yml`

## Install CLI

Global install:

```bash
npm install -g @runfabric/cli
runfabric --help
```

Beta channel:

```bash
npm install -g @runfabric/cli@beta
runfabric --help
```

Without global install:

```bash
npx @runfabric/cli@latest --help
```

If the npm package is not published yet in your environment, use source/link setup in `docs/REPO_DEVELOPMENT.md`.

## Install Provider Adapters (Only What You Use)

`@runfabric/cli` loads provider adapters dynamically. Install only the providers your project needs.

Example (AWS only):

```bash
npm install -D @runfabric/provider-aws-lambda
```

Example (AWS + Cloudflare):

```bash
npm install -D @runfabric/provider-aws-lambda @runfabric/provider-cloudflare-workers
```

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
- `docs/PROVIDER-SETUP.md`

Run lifecycle:

```bash
runfabric doctor -c ./my-api/runfabric.yml
runfabric plan -c ./my-api/runfabric.yml
runfabric build -c ./my-api/runfabric.yml
runfabric deploy -c ./my-api/runfabric.yml
```

Run local provider-mimic server:

```bash
cd my-api
npm run call:local
curl -i http://127.0.0.1:8787/hello
# stop server: Ctrl+C or type 'exit' and press Enter
```

`call:local` now runs in watch mode by default in scaffolded projects.

## Framework Wrappers (Express, Fastify, NestJS)

`@runfabric/runtime-node` exposes one wrapper helper so existing framework apps can be used as a `UniversalHandler`.

```ts
import type { UniversalHandler } from "@runfabric/core";
import { createHandler } from "@runfabric/runtime-node";

// Auto-detects Nest app (getHttpAdapter), Fastify instance (inject), or Express app function:
export const handler: UniversalHandler = createHandler(appOrFastifyOrNestApp);
```

## Core Commands

- `runfabric init`
- `runfabric docs check|sync`
- `runfabric doctor`
- `runfabric plan`
- `runfabric build`
- `runfabric package`
- `runfabric deploy`
- `runfabric call-local`
- `runfabric dev`
- `runfabric migrate`
- `runfabric invoke`
- `runfabric logs`
- `runfabric traces`
- `runfabric metrics`
- `runfabric remove`
- `runfabric compose plan|deploy`
- `runfabric state pull|list|backup|restore|force-unlock|migrate|reconcile`

## Security Scanning

- Snyk workflow: `.github/workflows/snyk.yml`
- Local command: `pnpm run security:snyk:test` (requires `SNYK_TOKEN`)

`invoke` and `logs` resolve project context from the current working directory; run them from the target project root.

Full command reference: `docs/site/command-reference.md`

## Providers

- `aws-lambda`
- `gcp-functions`
- `azure-functions`
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

## Docs

- `docs/QUICKSTART.md`
- `docs/HANDLER_SCENARIOS.md`
- `docs/CREDENTIALS.md`
- `docs/CREDENTIALS_MATRIX.md`
- `docs/PROVIDER-SETUP.md`
- `docs/ARCHITECTURE.md`
- `docs/REPO_DEVELOPMENT.md`
- `docs/CI_TEMPLATES.md`
- `docs/PLUGIN_API.md`
- `docs/RUNFABRIC_YML_REFERENCE.md`
- `docs/EXAMPLES_MATRIX.md`
- `docs/EXAMPLE_VALIDATION.md`
- `docs/COMPARISON.md`
- `docs/site/README.md`

## Community

- `SOCIAL_ACCOUNTS_TODO.md`

## Contributing

- `CONTRIBUTING.md`
- `CODE_OF_CONDUCT.md`
- `AGENTS.md`

## Versioning

- `VERSIONING.md`

## License

MIT (`LICENSE`)

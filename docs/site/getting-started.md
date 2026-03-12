# Getting Started

Scope note:

- runfabric is a CLI-first serverless framework.
- It uses `runfabric.yml` (not `serverless.yml`).
- Current release train is Node-first (`runtime: nodejs`).

1. Install dependencies:

```bash
corepack enable
corepack prepare pnpm@10.5.2 --activate
pnpm install
```

2. Pick a starter template:

```bash
pnpm run runfabric -- init --template api --provider aws-lambda --state-backend local --dir ./my-service
```

Available templates:

- `api`
- `worker`
- `queue`
- `cron`

`init` supports interactive prompts for template, provider, state backend, and language.

Template scope note:

- `init` currently scaffolds only `api|worker|queue|cron`.
- For `storage|eventbridge|pubsub` flows, scaffold with `worker` and then edit `triggers` in `runfabric.yml`.

Provider IDs (copy/paste):

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

State backend defaults to `local` unless explicitly set with `--state-backend`.

3. Set provider credentials (see `docs/CREDENTIALS.md`).
4. If using non-local state backend, set backend credentials too (see `docs/STATE_BACKENDS.md`).
5. Use `docs/CREDENTIALS_MATRIX.md` for one-page provider + state credential lookup.
6. Use `docs/EXAMPLE_VALIDATION.md` when producing provider/trigger/state example sets.
7. Run lifecycle commands:

```bash
pnpm run runfabric -- doctor -c ./my-service/runfabric.yml
pnpm run runfabric -- plan -c ./my-service/runfabric.yml
pnpm run runfabric -- build -c ./my-service/runfabric.yml
pnpm run runfabric -- package -c ./my-service/runfabric.yml
pnpm run runfabric -- deploy -c ./my-service/runfabric.yml
```

README drift guardrails:

```bash
pnpm run runfabric -- docs check -c ./my-service/runfabric.yml
pnpm run runfabric -- docs sync -c ./my-service/runfabric.yml
```

Run `invoke` and `logs` from the target project directory (they resolve project context from current working directory).

8. Optional migration from an existing Serverless Framework file:

```bash
pnpm run runfabric -- migrate --input ./serverless.yml --output ./runfabric.yml --json
```

Handler patterns and framework wrappers:

- `docs/HANDLER_SCENARIOS.md`
- `examples/handler-scenarios/README.md`

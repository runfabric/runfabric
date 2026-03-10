# runfabric

**alternate of Serverless Framework**

`runfabric` is a multi-provider serverless framework package for teams that want a single config and command flow across cloud providers.

## Why runfabric

- One service config (`runfabric.yml`) for multiple providers.
- Portability diagnostics before deployment.
- Package-first usage: local shell credentials, no mandatory CI deploy flow.
- Works as an alternative to [Serverless Framework](https://github.com/serverless/serverless).

## Features

- Unified workflow: `doctor -> plan -> build -> deploy -> remove`
- Provider credential schemas + doctor checks
- Stage-aware state tracking under `.runfabric/state/<service>/<stage>/<provider>.state.json`
- Provider deployment receipts under `.runfabric/deploy/<provider>/deployment.json`
- `invoke`, `logs`, and `destroy` support across provider adapters
- Optional rollback-on-failure during deploy: `RUNFABRIC_ROLLBACK_ON_FAILURE=1`
- Function-level lifecycle (`package`, `deploy function`, `remove`)
- Compose orchestration with dependency order + shared endpoint outputs
- Starter scaffolds: `runfabric init --template api|worker|queue|cron`

## Install

```bash
corepack enable
corepack prepare pnpm@10.5.2 --activate
pnpm install
```

Run CLI from repo:

```bash
pnpm run runfabric -- --help
```

## Quick Start

Generate a project:

```bash
pnpm run runfabric -- init --template api --dir ./my-api --provider cloudflare-workers
```

Run lifecycle commands:

```bash
pnpm run runfabric -- doctor -c ./my-api/runfabric.yml
pnpm run runfabric -- plan -c ./my-api/runfabric.yml
pnpm run runfabric -- build -c ./my-api/runfabric.yml
pnpm run runfabric -- deploy -c ./my-api/runfabric.yml
```

## Providers

Supported provider adapters:

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

### Deploy modes

- Simulated mode (default): deterministic endpoint + local receipt.
- Real mode (opt-in): provider command/API response parsing.

Global or provider flag:

- `RUNFABRIC_REAL_DEPLOY=1`
- `RUNFABRIC_<PROVIDER>_REAL_DEPLOY=1`

Provider real deploy command envs (JSON output expected):

- `RUNFABRIC_AWS_DEPLOY_CMD`
- `RUNFABRIC_GCP_DEPLOY_CMD`
- `RUNFABRIC_AZURE_DEPLOY_CMD`
- `RUNFABRIC_VERCEL_DEPLOY_CMD`
- `RUNFABRIC_NETLIFY_DEPLOY_CMD`
- `RUNFABRIC_ALIBABA_DEPLOY_CMD`
- `RUNFABRIC_DIGITALOCEAN_DEPLOY_CMD`
- `RUNFABRIC_FLY_DEPLOY_CMD`
- `RUNFABRIC_IBM_DEPLOY_CMD`

Cloudflare uses direct API mode with:

- `RUNFABRIC_CLOUDFLARE_REAL_DEPLOY=1`

Destroy command env pattern:

- `RUNFABRIC_<PROVIDER>_DESTROY_CMD`

## Compose

Compose deploy exports endpoint outputs as env vars:

- `RUNFABRIC_OUTPUT_<SERVICE>_<PROVIDER>_ENDPOINT`

References:

- `examples/hello-http/runfabric.compose.yml`
- `examples/compose-contracts/runfabric.compose.yml`

## Docs

Primary docs:

- `docs/QUICKSTART.md`
- `docs/CREDENTIALS.md`
- `docs/PROVIDER-SETUP.md`
- `docs/ARCHITECTURE.md`
- `docs/site/README.md`
- `docs/site/command-reference.md`
- `docs/site/provider-onboarding.md`

Release docs:

- `docs/RELEASE.md`
- `RELEASE_PROCESS.md`

## Community

Planned channels checklist:

- `SOCIAL_ACCOUNTS_TODO.md`

## Contributing

- `CONTRIBUTING.md`
- `CODE_OF_CONDUCT.md`
- `AGENTS.md`

## Versioning

- `VERSIONING.md`

## License

MIT (`LICENSE`)

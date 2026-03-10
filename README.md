# runfabric

**alternate of Serverless Framework**

`runfabric` is a multi-provider serverless deployment framework focused on portability-first planning, provider adapters, and package-friendly local deployment workflows.

## Table of Contents

- [Why runfabric](#why-runfabric)
- [Features](#features)
- [Install](#install)
- [Quick Start](#quick-start)
- [Provider Support](#provider-support)
- [CLI Commands](#cli-commands)
- [Credentials](#credentials)
- [Documentation](#documentation)
- [Roadmap](#roadmap)
- [Community](#community)
- [Contributing](#contributing)
- [Versioning](#versioning)
- [License](#license)

## Why runfabric

`runfabric` gives you one service config (`runfabric.yml`) and one CLI flow (`doctor -> plan -> build -> deploy`) across multiple serverless providers.

Project goals:

- Keep deployment intent provider-agnostic.
- Surface portability and capability gaps before deployment.
- Let package users deploy with environment credentials (local shell or `.env`) without mandatory CI coupling.

## Features

- Unified YAML config (`runfabric.yml`) with stage-aware overrides.
- Portability diagnostics for triggers and platform primitives.
- Provider-specific build artifacts under `.runfabric/build/<provider>/<service>/`.
- Provider credential schemas with `runfabric doctor` checks.
- Stage-aware local deployment state backend:
  - `.runfabric/state/<service>/<stage>/<provider>.state.json`
- Lifecycle hook extension points:
  - `beforeBuild`, `afterBuild`, `beforeDeploy`, `afterDeploy`
- Compose orchestration with dependency ordering and shared endpoint outputs:
  - `RUNFABRIC_OUTPUT_<SERVICE>_<PROVIDER>_ENDPOINT`
- Function lifecycle commands:
  - `package`
  - `deploy function <name>`
  - `remove`
- Explicit deploy exit codes:
  - `0` all succeeded
  - `2` partial failures
  - `1` all failed

## Install

### Local Development (Monorepo)

```bash
corepack enable
corepack prepare pnpm@10.5.2 --activate
pnpm install
```

### Run CLI From Repo

```bash
pnpm run runfabric -- --help
```

## Quick Start

Fastest onboarding path:

1. Use `examples/hello-http/runfabric.quickstart.yml`.
2. Set Cloudflare credentials (`CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ACCOUNT_ID`).
3. Run:

```bash
pnpm run runfabric -- doctor -c examples/hello-http/runfabric.quickstart.yml
pnpm run runfabric -- plan -c examples/hello-http/runfabric.quickstart.yml
pnpm run runfabric -- build -c examples/hello-http/runfabric.quickstart.yml
pnpm run runfabric -- deploy -c examples/hello-http/runfabric.quickstart.yml
```

Full guide: `docs/QUICKSTART.md`

## Provider Support

Current provider adapter packages:

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

Notes:

- Cloudflare Workers supports opt-in real API deploy mode (`RUNFABRIC_CLOUDFLARE_REAL_DEPLOY=1`).
- Other providers currently produce simulated deploy receipts and endpoint outputs.

## CLI Commands

- `runfabric init`
- `runfabric doctor`
- `runfabric plan`
- `runfabric build`
- `runfabric package`
- `runfabric deploy`
- `runfabric deploy function <name>`
- `runfabric deploy-function <name>` (alias)
- `runfabric remove`
- `runfabric compose plan`
- `runfabric compose deploy`
- `runfabric invoke`
- `runfabric logs`
- `runfabric providers`
- `runfabric primitives`

## Credentials

Package usage is env-first. You can pass provider credentials by:

- shell `export`
- `.env` + `source`
- one-off command prefix

Provider-wise variable list and examples:

- `docs/CREDENTIALS.md`
- `docs/PROVIDER-SETUP.md`

## Documentation

- `docs/ARCHITECTURE.md`
- `docs/QUICKSTART.md`
- `docs/CREDENTIALS.md`
- `docs/PROVIDER-SETUP.md`
- `docs/MIGRATION.md`
- `docs/RELEASE.md`
- `docs/TODO.md`
- `CHANGELOG_POLICY.md`
- `CHANGELOG.md`
- `RELEASE_PROCESS.md`

## Roadmap

Prioritized pending roadmap is tracked in `docs/TODO.md`.

## Community

Community channels are being prepared. Setup checklist:

- `SOCIAL_ACCOUNTS_TODO.md`

Planned channels:

- X / Twitter
- Community Slack
- Serverless Meetups
- Stack Overflow
- Facebook
- Contact Us

## Contributing

See `CONTRIBUTING.md` for setup, workflow, and PR expectations.

Code of conduct: `CODE_OF_CONDUCT.md`

## Versioning

See `VERSIONING.md`.

## License

MIT - see `LICENSE`.

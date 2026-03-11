# runfabric

**Alternate of Serverless Framework**

`runfabric` is a multi-provider serverless framework package. It gives you one config and one CLI workflow across cloud providers.

## Why runfabric

- Single service config: `runfabric.yml`
- Provider portability checks before deploy
- Provider credential schema + `doctor` validation
- Works as an alternative to [Serverless Framework](https://github.com/serverless/serverless)

## Features

- Unified lifecycle: `doctor -> plan -> build -> deploy -> remove`
- Interactive project scaffold: `runfabric init`
- Local provider-mimic execution: `runfabric call-local`
- Multi-provider invoke/logs/remove support
- Compose deploy orchestration with dependency ordering
- Stage-aware local state and deployment receipts

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

## Quick Start

Create a new project:

```bash
runfabric init --dir ./my-api
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
- `runfabric doctor`
- `runfabric plan`
- `runfabric build`
- `runfabric deploy`
- `runfabric call-local`
- `runfabric invoke`
- `runfabric logs`
- `runfabric remove`
- `runfabric compose plan|deploy`

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
- State file: `.runfabric/state/<service>/<stage>/<provider>.state.json`

## Docs

- `docs/QUICKSTART.md`
- `docs/HANDLER_SCENARIOS.md`
- `docs/CREDENTIALS.md`
- `docs/PROVIDER-SETUP.md`
- `docs/ARCHITECTURE.md`
- `docs/REPO_DEVELOPMENT.md`
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

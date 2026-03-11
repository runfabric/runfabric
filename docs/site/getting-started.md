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
pnpm run runfabric -- init --template api --dir ./my-service
```

Available templates:

- `api`
- `worker`
- `queue`
- `cron`

3. Set provider credentials (see `docs/CREDENTIALS.md`).
4. Run lifecycle commands:

```bash
pnpm run runfabric -- doctor -c ./my-service/runfabric.yml
pnpm run runfabric -- plan -c ./my-service/runfabric.yml
pnpm run runfabric -- build -c ./my-service/runfabric.yml
pnpm run runfabric -- package -c ./my-service/runfabric.yml
pnpm run runfabric -- deploy -c ./my-service/runfabric.yml
```

Run `invoke` and `logs` from the target project directory (they resolve project context from current working directory).

5. Optional migration from an existing Serverless Framework file:

```bash
pnpm run runfabric -- migrate --input ./serverless.yml --output ./runfabric.yml --json
```

Handler patterns and framework wrappers:

- `docs/HANDLER_SCENARIOS.md`
- `examples/handler-scenarios/README.md`

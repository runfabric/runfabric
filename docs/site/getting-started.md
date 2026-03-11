# Getting Started

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
pnpm run runfabric -- deploy -c ./my-service/runfabric.yml
```

Handler patterns and framework wrappers:

- `docs/HANDLER_SCENARIOS.md`
- `examples/handler-scenarios/README.md`

# Repository Development

This document covers workspace-level commands and repository structure for contributors.

## Prerequisites

- Node.js `>= 20`
- Corepack enabled
- pnpm `10.5.2`

## Local Setup

```bash
corepack enable
corepack prepare pnpm@10.5.2 --activate
pnpm install
```

## Run CLI From Source

```bash
pnpm run runfabric -- --help
```

Create a global local link:

```bash
npm run link:cli
runfabric --help
```

`link:cli` builds only the CLI runtime graph (`core`, `planner`, `builder`, `runtime-node`, `cli`) and does not build provider adapters.

Expected warning with `pnpm link --global`:

- pnpm may warn that `@runfabric/cli` has optional provider peer dependencies not resolved in the linked target project.
- This is expected for runfabric's modular provider model.
- Install only the provider adapters used by the target project, for example:

```bash
cd /path/to/your-service
pnpm add -D @runfabric/provider-aws-lambda
```

Remove global local link:

```bash
npm run unlink:cli
```

If `runfabric` is not found in a new terminal:

```bash
export PNPM_HOME="${PNPM_HOME:-$HOME/.pnpm}"
export PATH="$PNPM_HOME:$PATH"
```

## Workspace Commands

- `npm run check:syntax`
- `npm run check:capabilities`
- `npm run check:docs-sync`
- `npm run check:compatibility`
- `npm test`
- `pnpm -r --if-present run build`
- `pnpm -r --if-present run typecheck`
- `pnpm run security:snyk:test` (requires `SNYK_TOKEN`)
- `npm run release:check`

## Repository Structure

- `apps/cli`
  - CLI entrypoints and command implementations
- `packages/core`
  - shared contracts, credential/state abstractions, provider helpers
- `packages/planner`
  - config parsing, validations, portability planning
- `packages/builder`
  - artifact assembly by provider
- `packages/runtime-node`
  - runtime adapter implementations
- `packages/provider-*`
  - provider-specific adapters
- `examples`
  - sample projects and compose examples
- `tests`
  - unit and integration tests
- `scripts`
  - utility, validation, and release scripts
- `docs`
  - product and contributor documentation

## Related Docs

- `CONTRIBUTING.md`
- `docs/ARCHITECTURE.md`
- `docs/CREDENTIALS.md`
- `docs/RELEASE.md`
- `RELEASE_PROCESS.md`

# Frontend V1 Plan (Registry-centric: API + SPA in one container)

This plan defines the **registry-centric** frontend model:

- The **registry** codebase owns both the API and the UI. Frontend lives under `registry/web/` (or `registry/frontend/`).
- The frontend is a **static/SPA** build — cache-friendly, suitable for CDN and long-lived cache headers.
- **Docs are split by audience:** (1) **Extension/plugin development** docs are surfaced in the registry UI; (2) **CLI** and general RunFabric usage docs live elsewhere (separate site or section) and are linked from the registry UI as needed.
- **One Docker container** builds and runs both the registry API and the static frontend; configurable via existing registry config (and env).

## Goals

- Single deployable: one image for registry API + marketplace + extension-dev docs + auth UX.
- Clear ownership: registry repo/team owns API and the UI that consumes it.
- Cache-friendly static SPA; API remains dynamic.
- Docs split so extension developers get relevant content in the registry UI; CLI users get dedicated CLI docs elsewhere.

## Implementation status (Phase 15)

Implemented in this repo:

- Frontend app: `registry/web/` (static SPA build).
- Registry server static serving via `--web-dir` / `REGISTRY_WEB_DIR` / `server.web_dir`.
- Build artifact: `registry/web/dist` (served by the same registry process as API routes).
- Docs loader: `registry/web/lib/docs` reads extension-dev docs from `docs/developer` and emits `docs-index.json`.
- One-image deployment: `registry/Dockerfile` builds Go API + SPA and serves both.

## Repository layout (target)

```text
registry/
├── cmd/registry/           # existing
├── internal/               # existing API
├── web/                    # or frontend/ — SPA (marketplace + extension-dev docs + auth)
│   ├── app/                # or src/ depending on stack (e.g. Next static export / Vite)
│   ├── components/
│   ├── lib/
│   │   ├── docs/           # loader for extension-dev docs
│   │   └── registry/       # API client for marketplace
│   ├── content/            # optional: extension-dev markdown here, or map from docs/
│   ├── public/
│   └── package.json
├── configs/
└── Dockerfile              # build Go + build SPA; serve both
```

Docs elsewhere (e.g. repo-root `docs/`):

- **Extension/plugin development** — source in `docs/developer/` or `docs/extension-dev/`; built into or loaded by registry UI.
- **CLI / general** — `docs/user/` or equivalent; hosted separately or linked from registry UI.

## Registry UI scope

The registry UI includes:

- **Extension-development docs** — Contract, catalog, registry usage, testing, publishing (single source of truth, no duplicate trees).
- **Marketplace** — Catalog, extension detail, versions, advisories, trust, install UX (copyable commands, etc.).
- **Auth** — Login, tokens, SSO UX as needed (OIDC flows; registry validates Bearer tokens).

It does **not** include full RunFabric CLI/config docs; those are a separate concern and linked where relevant.

## Route surface (V1)

- `/` — landing or dashboard
- `/docs` — extension-dev docs index
- `/docs/[...slug]` — extension-dev doc pages
- `/extensions` — marketplace catalog
- `/extensions/[id]` — extension detail
- `/extensions/[id]/versions/[version]` — version detail
- `/publishers/[publisher]` — publisher page
- `/search` — unified search (docs + marketplace)
- `/auth` — login/tokens UX (if needed)

## Frontend app structure (conceptual)

```text
registry/web/
├── app/                    # or src/ (e.g. Next.js app/ or Vite src/)
│   ├── page.tsx            # /
│   ├── docs/
│   │   ├── [[...slug]]/page.tsx
│   │   ├── layout.tsx
│   │   └── page.tsx
│   ├── extensions/
│   │   ├── page.tsx
│   │   ├── [id]/page.tsx
│   │   ├── [id]/versions/[version]/page.tsx
│   │   └── ...
│   ├── publishers/[publisher]/page.tsx
│   ├── search/page.tsx
│   ├── auth/                # login, tokens
│   │   └── page.tsx
│   └── layout.tsx
├── components/
│   ├── docs/
│   ├── marketplace/
│   └── shared/
├── lib/
│   ├── docs/                # loader, slugs, extension-dev content mapping
│   └── registry/            # API client
├── content/                 # optional: extension-dev markdown, or reference docs/
├── public/
└── package.json
```

## Runtime and deployment

- **Build:** Dockerfile (or Makefile) builds Go binary and runs frontend build (e.g. `npm run build`); output is binary + static assets (e.g. `dist/` or `out/`).
- **Serve:** Registry server serves API on `/v1/...` (or existing prefix) and static SPA on `/` (and optionally `/docs` for pre-rendered or SPA-routed doc pages). SPA handles client-side routes; API is same-origin, no CORS for same deployment.
- **Config:** Existing registry config (`config.yaml`, env) drives API and can drive UI (auth endpoints, registry URL, feature flags). No separate "web config" required unless UI-specific options are added.

## Docs mapping strategy

- **Extension-dev docs:** Single source of truth in `docs/` (e.g. `docs/developer/EXTENSION_DEVELOPMENT_GUIDE.md`, `ADDON_CONTRACT.md`, `EXTERNAL_EXTENSIONS_PLAN.md`) or under `registry/web/content/`. Loader maps markdown to route slugs (e.g. `/docs/extension-guide`, `/docs/addon-contract`). No copy of doc tree inside `registry/web/` unless chosen as the canonical location.
- **CLI docs:** Remain in `docs/user/` (or equivalent); hosted on separate site or linked from registry UI (e.g. "CLI reference", "Quickstart") so plugin developers can jump to CLI when needed.

## Non-goals / anti-patterns

- No separate top-level `web/` app that duplicates registry UI scope.
- No duplicate extension-dev doc trees (e.g. no `registry/web/docs-copy/` that mirrors `docs/`).
- No registry business logic inside the frontend; UI only calls registry APIs and renders extension-dev content.

## Completion criteria

- Registry Docker image builds and runs with API + static frontend; one container, configurable.
- Registry UI serves marketplace (from registry APIs), extension-dev docs (from loader + single source), and auth UX.
- Docs split is documented: extension-dev in registry UI; CLI/general elsewhere and linked.
- Unified search covers extension-dev docs and marketplace entities.
- Frontend is static/SPA and cache-friendly.

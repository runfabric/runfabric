# Registry Web UI

`registry/web` is the registry-centric frontend app.

Scope:

- extension-development docs (`/docs`, `/docs/[...slug]`)
- marketplace pages (`/extensions`, `/extensions/[id]`, version detail)
- publisher and unified search (`/publishers/[publisher]`, `/search`)
- auth/token helper (`/auth`) with configurable SSO redirect URL
- light/dark theme toggle with persisted preference

Build:

```bash
cd registry
npm --prefix web run build
```

Output:

- `registry/web/dist` (static assets + `index.html` + `docs-index.json`)

Docs source of truth:

- `docs/developer/*` via `registry/web/lib/docs` loader (no mirrored markdown tree in `registry/web`).

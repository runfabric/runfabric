# Release Process

This is the root runbook for shipping `runfabric`.

## 1. Version + Changelog

- Decide release version per `VERSIONING.md`.
- Update versions.
- Update `CHANGELOG.md` section `## [<version>]`.

## 2. Release Notes + Signature

Create release notes file:

- `release-notes/<version>.md`

Sign file:

```bash
RELEASE_NOTES_SIGNING_KEY="<key>" pnpm run release:notes:sign -- --version <version>
```

Verify signature + changelog alignment:

```bash
RELEASE_NOTES_SIGNING_KEY="<key>" pnpm run release:notes:verify -- --version <version>
```

## 3. Validation

```bash
pnpm install --frozen-lockfile
pnpm run release:check
```

## 4. Automation

Run `.github/workflows/release.yml` with:

- `version=<version>`
- `publish=true`
- `dry_run=false` for actual publish

Use `dry_run=true` first when validating process.

## 5. Post-release

- Confirm npm packages are available.
- Confirm git tag and GitHub release are created.
- Run quick smoke from `docs/QUICKSTART.md`.

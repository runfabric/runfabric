# Release Process

This is the root runbook for shipping `runfabric`.

## 1. Version + Changelog

- Decide release version per `VERSIONING.md`.
- For beta use semver pre-release starting at `-beta.0` (example: `0.2.0-beta.0`).
- Update versions.
- Update `CHANGELOG.md` section `## [<version>]`.

## 2. Release Notes + Signature

Create release notes file:

- `release-notes/<version>.md`
- or scaffold it:

```bash
pnpm run release:notes:create -- --version <version>
```

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
pnpm run check:compatibility
pnpm run release:check
```

## 4. npm Credentials

CI/GitHub Actions:
- Use a granular npm access token with package publish permission for `@runfabric/*`.
- Enable bypass 2FA for publish on that token.
- Save it as repository secret `NPM_TOKEN`.

`release:publish` now injects `NPM_TOKEN` into a temporary npm user config for the publish process, so a repository `.npmrc` token stanza is not required.

Local publish with 2FA account:
- Option A: use the same bypass token in `NPM_TOKEN`.
- Option B: pass one-time code at publish time:

```bash
NPM_TOKEN="<token>" NPM_OTP="<6-digit-code>" pnpm run release:publish -- --tag beta
```

## 5. Automation

Run `.github/workflows/release.yml` with:

- `version=<version>`
- `publish=true`
- `dry_run=false` for actual publish
- `dist_tag=latest` for stable or `dist_tag=beta` for beta channel

Use `dry_run=true` first when validating process.

CLI equivalents:

```bash
pnpm run release:publish:dry-run -- --tag beta
pnpm run release:publish -- --tag beta
```

## 6. Post-release

- Confirm npm packages are available.
- For beta verify install path:

```bash
npx @runfabric/cli@beta --help
```

- Confirm git tag and GitHub release are created.
- Run quick smoke from `docs/QUICKSTART.md`.

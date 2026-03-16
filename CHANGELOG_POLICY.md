# Changelog Policy

This policy defines how `runfabric` maintains `CHANGELOG.md` for releases.

## Objectives

- Make release impact obvious for users and maintainers.
- Distinguish breaking changes from additive fixes/features.
- Keep package release notes synchronized across the monorepo.

## Changelog Scope

`CHANGELOG.md` should include all user-visible changes affecting:

- CLI behavior
- `runfabric.yml` schema
- Provider adapter behavior
- Build/deploy/state semantics
- Credential requirements
- Documentation with operational impact

## Entry Format

Each release entry should include:

- version and release date
- summary paragraph
- sections:
  - Added
  - Changed
  - Fixed
  - Deprecated
  - Removed
  - Security
- migration notes for any breaking changes

## Breaking Changes

When a release contains breaking changes:

- add a clear `Breaking` callout in the release section
- link to `docs/MIGRATION.md`
- include concrete before/after examples

## Monorepo Coordination

For multi-package releases:

- summarize cross-package impact in a single root changelog entry
- note package-level highlights when needed
- ensure publish order in `RELEASE_PROCESS.md` is respected

## Authoring Workflow

1. Collect merged PRs/issues for the target release.
2. Classify each change under the correct section.
3. Draft migration notes for breaking changes.
4. Validate links to docs and examples.
5. Final review during release checklist execution.

## Quality Bar

- No vague entries like "misc fixes".
- Prefer user-impact statements over internal implementation detail.
- Include affected command/file/schema names where useful.

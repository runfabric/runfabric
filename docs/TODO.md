# Project TODO

This file tracks only pending work, prioritized from highest impact to lowest.

## P1 - Production Deployment Completeness

- Implement real deploy execution paths for non-Cloudflare providers (AWS, GCP, Azure, Vercel, Netlify, Alibaba FC, DigitalOcean Functions, Fly Machines, IBM OpenWhisk).
- Replace simulated endpoint generation with provider API/CLI response parsing for all providers.
- Add provider-specific rollback and partial-failure recovery semantics during deploy/remove operations.

## P2 - Runtime And Command Behavior

- Implement real `invoke` behavior for providers that currently return placeholder responses.
- Implement real `logs` retrieval for providers that currently report "not implemented".
- Expand runtime support beyond `nodejs` where provider capability matrix allows it.
- Add explicit provider destroy support where `remove` currently only cleans local artifacts/state.

## P3 - Package Quality And Release Hardening

- Replace per-package placeholder `lint`/`test` scripts with actual package-level checks.
- Add release tagging and npm publish automation workflow built around `release:check`.
- Add signed release notes process tied to `CHANGELOG.md` updates.

## P4 - Ecosystem And Adoption

- Add opinionated starter templates (`api`, `worker`, `queue`, `cron`) using `runfabric init`.
- Publish reference compose examples with cross-service contracts and output conventions.
- Add docs site/navigation for quick provider onboarding and command reference.

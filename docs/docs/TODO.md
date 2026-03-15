# Project TODO

Only pending work is listed here. Completed items are removed.

**Phased sub-tasks and Go core + wrappers alignment:** see [ROADMAP_PHASES.md](./ROADMAP_PHASES.md) for detailed phase breakdown, doc touchpoints, and sync with the final product (core engine in Go, Node/Python SDKs as wrappers).

## Phase 1: CLI Feature Parity

Align the local Go binary exactly with the features described in `docs/site/command-reference.md`.

- [x] Bootstrapped basic command matrix skeleton
- [x] Implement `runfabric init` logic and scaffolding template capabilities
- [x] Implement `runfabric docs check|sync`
- [x] Implement `runfabric build` & `runfabric package` adapters for function asset generation
- [x] Implement `runfabric call-local` with dev web server and file watching
- [x] Implement `runfabric dev` for live-sync workflows
- [x] Implement `runfabric migrate` logic to port Serverless Framework setups
- [x] Implement metric, trace, and logs integration
- [x] Implement `runfabric compose` graph capability mapping
- [x] Implement `runfabric state` backup/restore/migrate mechanics
- [x] Implement primitives and provider introspection commands (`runfabric providers`, `runfabric primitives`)

## Phase 2: Provider & Trigger Capability Matrix

Build the core backend mechanisms to support the targets published in `docs/docs/EXAMPLES_MATRIX.md`.

- [x] Implement `aws-lambda` (http, cron, queue, storage, eventbridge)
- [x] Implement `gcp-functions` (http, cron, queue, storage, pubsub)
- [x] Implement `azure-functions` (http, cron, queue, storage)
- [x] Implement `kubernetes` (http, cron)
- [x] Implement `cloudflare-workers` (http, cron)
- [x] Implement `vercel` (http, cron)
- [x] Implement `netlify` (http, cron)
- [x] Implement `alibaba-fc` (http, cron, queue, storage)
- [x] Implement `digitalocean-functions` (http, cron)
- [x] Implement `fly-machines` (http)
- [x] Implement `ibm-openwhisk` (http, cron)

## Phase 3: Node SDK & Wrapper

Standardize the `npm` installation path so end-users can seamlessly execute the multi-runtime core engine.

- [x] Configure `@runfabric/cli` NPM structure (`package.json`, `index.js`, `cli.js`)
- [x] Set up automated release actions for fetching OS-specific binaries within the `postinstall` step
- [x] Implement dynamic Node wrapper bindings for SDK programmatic usage
- [x] Prepare registry publishing workflows & credentials for `npm publish`
- [x] Add runtime tests to ensure cross-platform compatibility without manual Go installations

## Phase 4: Python SDK & Wrapper

Standardize the `pip` installation path so end-users can seamlessly execute the multi-runtime core engine in Python environments.

- [x] Configure `runfabric` PIP structure (`setup.py`, `__init__.py`, `cli.py`)
- [x] Set up auto-fetching of compiled binaries upon `pip install` via custom builder logic inside `setup.py`
- [x] Implement dynamic Python wrapper bindings to call programmatic SDK methods
- [x] Prepare registry publishing workflows to PiPy
- [x] Add runtime structural tests to validate Python 3 integration points

## Phase 5: Code Quality and Consistency

- [ ] Clean duplicate and redundant code; extract shared provider/config helpers
- [ ] Organize files, types, and business logic (clear separation from CLI/adapters)
- [x] Fix bugs and validation issues: backend.kind now accepts local, s3, gcs, azblob, postgres (was only local/aws-remote); config-reference.md populated
- [ ] Run release:check and fix compatibility/syntax/contract issues

## Code vs docs alignment (gaps)

- [ ] **State subcommands**: Command reference documents `state pull`, `list`, `backup`, `restore`, `force-unlock`, `migrate`, `reconcile`. CLI now has these subcommands but they return "not yet implemented". Wire them to internal/state and backends to complete the feature.
- [x] **Command reference vs invoke/logs**: Updated command-reference to list `-c/--config`, `--function`, `--payload` for invoke and `-c`, `--function` for logs.

## Phase 6: Scaffolding by Language, Trigger, and Provider

- [x] Extend `runfabric init` with `--lang <node|ts|python|go>` and templates per (trigger × provider × language)
- [x] Restrict template options to Trigger Capability Matrix; update command-reference and examples docs
- [x] Add tests for non-interactive init across a subset of (lang, template, provider)

## Phase 7: Test Coverage and Quality Gates

- [x] Add unit and integration tests for planner, config, lifecycle, state, providers (capability matrix + init tests added)
- [x] Achieve and enforce ~95% coverage on critical paths; document in CONTRIBUTING/CI

## Code analysis: internal/cli/init.go

Findings from security/performance/code-quality review of the init command and provider selection flow.

### Security

- [x] **YAML injection via service name**: `o.Service` is now passed through `yamlQuoted()` so unsafe characters (newlines, colons, etc.) are double-quoted and escaped in generated `runfabric.yml`.

### Performance

- [x] **Single-byte reads in promptSelectArrow**: Kept single-byte read for key handling; added 8-byte buffer for ESC sequence consumption to allow future batching if needed.

### Code quality

- [x] **ESC sequence handling**: ESC sequences are consumed until a terminating letter (A–Z, a–z) or `maxESCSeq` (16) bytes to avoid blocking on unknown sequences.
- [x] **Magic numbers**: Introduced `asciiESC` (27) and `asciiETX` (3) constants.
- [x] **Variable shadowing**: Renamed outer read result to `nn` and inner loop uses separate `nr`/`c` to avoid shadowing.

## Code analysis: providers/aws/triggers/storage.go (+ related)

Findings from security/performance/code-quality review of the AWS S3 storage trigger setup and related files (cmd/runfabric/main.go, internal/deploy/doc.go, providers/aws/adapter.go, providers/aws/triggers/storage.go, provider doc.go files).

### Security

- No issues identified in the provided code (bucket/ARN usage goes through AWS SDK; no injection or credential exposure).

### Performance

- [x] **Repeated S3 API calls per bucket**: Storage events are now grouped by bucket; each bucket is read once and written once (`GetBucketNotificationConfiguration` + `PutBucketNotificationConfiguration` per bucket). Lambda `AddPermission` remains per-event.

### Code quality

- [x] **Struct literal alignment**: `NotificationConfiguration` struct fields are aligned consistently (`TopicConfigurations`, `QueueConfigurations`, `EventBridgeConfiguration`).

## Code analysis: .github/workflows (ci.yml, release.yml)

Findings from security/performance/code-quality review of the GitHub Actions workflows.

### Security

- No issues identified (minimal permissions; no secret or token exposure beyond default GITHUB_TOKEN in release).

### Performance

- [x] **No Go module/build cache in CI**: Added `actions/cache@v4` for `~/.cache/go-build` and `~/go/pkg/mod` keyed by `go.sum`.

### Code quality

- [x] **Go version mismatch across workflows**: Aligned `release.yml` to use `go-version: "1.22"` to match `ci.yml`.

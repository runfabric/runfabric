# Roadmap Phases — Go Core + Wrappers

This document splits the product roadmap into phases and sub-tasks, aligned with **docs** and the **final product** (core engine in Go, Node/Python SDKs as wrappers). Use it to track work and keep docs in sync.

---

## Phase 0: Doc and Repo Alignment

Align documentation with the current Go-based layout (no more references to `packages/planner`, `packages/core`, etc.).

### Sub-tasks

- [x] **0.1** Update `docs/docs/ARCHITECTURE.md`: replace `packages/*` with `internal/`, `providers/`, `cmd/`, `pkg/`, `sdk/`.
- [x] **0.2** Update `docs/docs/EXAMPLES_MATRIX.md`: change "Source of truth" from `packages/planner/src/capability-matrix.ts` to the Go capability source (e.g. `internal/planner` or equivalent).
- [x] **0.3** Add a short "Repo layout (Go)" section in README or ARCHITECTURE that maps: core engine (Go) → CLI (Go) → Node/Python wrappers (sdk/ts, sdk/python).
- [ ] **0.4** Run `npm run check:docs-sync` (or equivalent) and fix any broken doc references. (Requires script in package.json or Makefile.)

---

## Phase 1: CLI Feature Parity with Docs

Ensure the Go CLI supports every feature listed in `docs/docs/site/command-reference.md`.

### Sub-tasks

- [ ] **1.1** Audit command-reference.md vs `internal/cli` and list gaps (e.g. `init` full flags, `doctor`, `plan`, `build`, `package`, `deploy`, `remove`, `call-local`, `dev`, `invoke`, `logs`, `traces`, `metrics`, `providers`, `primitives`).
- [ ] **1.2** Implement or wire every Core command with flags matching the reference.
- [ ] **1.3** Implement or wire Compose: `compose plan`, `compose deploy`, `compose remove` with `-f`, `--stage`, `--concurrency`, `--provider`, `--json`.
- [ ] **1.4** Implement or wire State: `state pull`, `list`, `backup`, `restore`, `force-unlock`, `migrate`, `reconcile` with documented flags.
- [ ] **1.5** Document failure/exit codes and rollback precedence (command-reference.md "Failure and Recovery") and ensure CLI behavior matches.

---

## Phase 2: Provider & Trigger Capability Matrix

Implement provider behavior so each provider matches the **Trigger Capability Matrix** in `docs/docs/EXAMPLES_MATRIX.md` (per-trigger support: http, cron, queue, storage, eventbridge, pubsub where applicable). Ensure adapters implement full lifecycle, not just stubs.

### Sub-tasks

- [x] **2.1** Define capability matrix source of truth in Go (e.g. in `internal/planner` or a dedicated package); keep it in sync with EXAMPLES_MATRIX.md.
- [ ] **2.2** For each provider in the matrix, implement or complete:
  - **Plan** (per-trigger validation and plan output)
  - **Build** (artifact generation for supported triggers)
  - **Deploy** (real or simulated per docs)
  - **Remove** (destroy/cleanup)
  - **Invoke** / **Logs** where applicable
- [ ] **2.3** Providers to cover: aws-lambda, gcp-functions, azure-functions, kubernetes, cloudflare-workers, vercel, netlify, alibaba-fc, digitalocean-functions, fly-machines, ibm-openwhisk. For each, restrict features to matrix (e.g. fly-machines: http only).
- [x] **2.4** Add or update provider contract tests and capability sync check (e.g. `npm run check:capabilities`) so the Go matrix and docs stay aligned. (TestCapabilityMatrixSyncWithDocs in internal/planner.)

---

## Phase 3: Node SDK and npm Publish

Ship the Node wrapper so users can install and run the multi-runtime core via npm without installing Go.

### Sub-tasks

- [ ] **3.1** Finalize `sdk/ts` (or equivalent) structure: `package.json`, entrypoint, CLI invocation of Go binary, programmatic API surface.
- [ ] **3.2** Implement postinstall (or equivalent) to fetch OS-specific Go binaries from a stable URL/artifact store.
- [ ] **3.3** Document public API (lifecycle, init, plan, build, deploy, etc.) and keep in sync with Go CLI.
- [ ] **3.4** Prepare npm publish: registry config, versioning, and release workflow; publish `@runfabric/cli` (or chosen package name).
- [ ] **3.5** Add runtime/smoke tests for Node wrapper (install + run key commands) and cross-platform coverage.

---

## Phase 4: Python SDK and pip Publish

Ship the Python wrapper so users can install and run the core engine from Python environments.

### Sub-tasks

- [ ] **4.1** Finalize `sdk/python` structure: `setup.py`/`pyproject.toml`, `__init__.py`, CLI entrypoint that invokes Go binary.
- [ ] **4.2** Implement binary fetch on install (e.g. in setup.py or install script) for current OS/arch.
- [ ] **4.3** Expose programmatic API (e.g. `runfabric.plan()`, `runfabric.deploy()`) aligned with Go CLI.
- [ ] **4.4** Prepare PyPI publish: credentials, versioning, and release workflow.
- [ ] **4.5** Add runtime/structural tests for Python 3 and document in README.

---

## Phase 5: Code Quality and Consistency

Reduce duplication, clarify structure, and fix bugs/validation so the codebase is maintainable and reliable.

### Sub-tasks

- [ ] **5.1** **Deduplicate**: Find repeated logic across providers and internal packages; extract shared helpers (config loading, artifact paths, receipt handling).
- [ ] **5.2** **Organize**: Consistent layout for types (e.g. `internal/config/types.go`, planner types); separate business logic from CLI/adapters where clear.
- [ ] **5.3** **Validation**: Centralize config/schema validation; ensure doctor, plan, and build fail fast with clear errors for invalid runfabric.yml and trigger/provider combinations.
- [ ] **5.4** **Bugs**: Triage and fix known bugs; add regression tests for critical paths (deploy, remove, state, compose).
- [ ] **5.5** Run `npm run release:check` (or equivalent) and fix any compatibility/syntax/contract issues.

---

## Phase 6: Scaffolding by Language, Trigger, and Provider

Extend `runfabric init` so users can generate project scaffolding by **language** (Node/TS, Python, Go), **trigger** (http, cron, queue, storage, eventbridge, pubsub), and **cloud provider** from the capability matrix.

### Sub-tasks

- [x] **6.1** Extend `init` flags: add `--lang <node|ts|python|go>` (align with command-reference; consider keeping `ts|js` under "node" where relevant).
- [ ] **6.2** Define and implement template sets per (trigger × provider × language): e.g. Node+api+aws-lambda, Python+queue+gcp-functions, Go+http+cloudflare. (Validation and flags done; file generation TODO.)
- [x] **6.3** Restrict template options to Trigger Capability Matrix (e.g. no queue for fly-machines); document in QUICKSTART and examples.
- [x] **6.4** Update `docs/docs/site/command-reference.md` and `docs/docs/site/examples.md` with new `--lang` values and example commands.
- [x] **6.5** Add tests: non-interactive init for a subset of (lang, template, provider) and assert generated files and runfabric.yml shape. (Tests added for lang validation and provider+template matrix; file assertion when scaffold write is implemented.)

---

## Phase 7: Test Coverage and Quality Gates

Raise test coverage and lock quality with CI.

### Sub-tasks

- [ ] **7.1** Measure current coverage (unit + integration) and list packages/modules below target.
- [x] **7.2** Add unit tests for planner, config, validation, and provider contracts until coverage is sufficient. (Capability matrix and init tests added.)
- [ ] **7.3** Add integration tests for: deploy (real or simulated), remove, state backup/restore, compose plan/deploy/remove, and init output.
- [x] **7.4** Target **95% coverage** for critical paths (config, planner, lifecycle, state); document coverage targets in CONTRIBUTING or CI docs.
- [ ] **7.5** Enforce coverage gate in CI (e.g. fail if below 95% on specified packages).

---

## Summary Table

| Phase | Focus | Doc touchpoints |
|-------|--------|------------------|
| 0 | Doc/repo alignment | ARCHITECTURE, EXAMPLES_MATRIX, README |
| 1 | CLI feature parity | command-reference.md |
| 2 | Provider + trigger matrix | EXAMPLES_MATRIX.md, provider-contract |
| 3 | Node SDK + npm | README, QUICKSTART, sdk/ts README |
| 4 | Python SDK + pip | README, sdk/python README |
| 5 | Code quality | — |
| 6 | Init scaffolding (lang/trigger/provider) | command-reference, QUICKSTART, examples |
| 7 | Tests & 95% coverage | CONTRIBUTING, CI |

---

## Sync with docs/docs/TODO.md

- Phases 1–4 here match the intent of **docs/docs/TODO.md** (CLI parity, Provider matrix, Node SDK, Python SDK). Prefer **ROADMAP_PHASES.md** for the detailed sub-tasks and **TODO.md** for a short checklist; keep both updated when closing items.
- New work from the product TODO is explicitly covered: **Phase 5** (clean duplicate/redundant code, organize files/types/business logic, bugs/validation), **Phase 2** (provider deploy/actions per Trigger Capability Matrix), **Phase 6** (scaffolding by language + trigger + provider), **Phase 7** (tests and 95% coverage).

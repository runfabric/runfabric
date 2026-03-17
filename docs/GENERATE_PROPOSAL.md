# runfabric generate — Proposal (P1)

**Status:** Proposed · **Priority:** P1

Add a new command group `runfabric generate` to scaffold artifacts inside an existing RunFabric project without forcing users to hand-edit config. **init** is bootstrap; **generate** makes the CLI useful during day-to-day development.

---

## 1. Executive summary

| Item | Description |
|------|-------------|
| **Proposal** | New command group: `runfabric generate` |
| **Primary goal** | Scaffold new artifacts in-project; reduce YAML drift; keep conventions consistent |
| **MVP** | `runfabric generate function <name>` with triggers: **http**, **cron**, **queue** |
| **Key behavior** | Read `runfabric.yml` → infer provider/language → validate trigger compatibility → generate handler file → safely patch config → support `--dry-run` and `--force` |

**Why it matters:** After the first scaffold, users today must manually create handler files, patch `runfabric.yml`, and remember provider trigger constraints. The CLI helps once, then becomes less useful. Generate closes that gap.

---

## 2. Problem statement

- **Current state:** RunFabric has `init` for project bootstrap and template generation; no in-project command for adding new functions over time.
- **Result:** Users hand-edit config and handlers, risking drift and inconsistency. Generate should make the CLI continue to matter after day one.

---

## 3. Product decision

- **Do NOT:** Full re-scaffold, dumb file copy, whole-config regeneration.
- **Do:** Config-aware, provider-aware, safe for existing projects, minimal but extensible.

---

## 4. CLI shape

### 4.1 Top-level

```bash
runfabric generate
runfabric generate function <name>
```

### 4.2 MVP

```bash
runfabric generate function hello --trigger http --route GET:/hello
```

### 4.3 Flags (MVP)

| Flag | Purpose |
|------|---------|
| `--trigger` | `http` \| `cron` \| `queue` |
| `--provider` | Optional provider override |
| `--lang` | `js` \| `ts` \| `go` \| `python` (only if project cannot infer) |
| `--entry` | Custom handler path |
| `--route` | HTTP route, e.g. `GET:/hello` |
| `--schedule` | Cron schedule |
| `--queue-name` | Queue name |
| `--dry-run` | Preview files + config diff without writing |
| `--force` | Overwrite generated file if it exists (never config) |
| `--json` | Machine-readable output |
| `--yes` | Skip prompts |

### 4.4 Future (out of MVP)

- `runfabric generate resource redis`
- `runfabric generate resource postgres`
- `runfabric generate addon sentry`
- `runfabric generate provider-override aws`
- `runfabric generate workflow image-pipeline`

---

## 5. MVP scope

**Ship only:** `runfabric generate function <name>`

**Support only:** http, cron, queue

**Out of scope for MVP:** resources, addons, provider overrides, workflows, plugin generators.

---

## 6. Functional behavior

### 6.1 Flow

1. Locate project root  
2. Load `runfabric.yml`  
3. Infer provider / language / source layout  
4. Validate trigger compatibility (existing provider capabilities)  
5. Build file generation plan + YAML patch plan  
6. If `--dry-run`: show plan and diff, exit  
7. Write file(s)  
8. Patch `runfabric.yml` safely  
9. Print next steps  

### 6.2 Example output

- **File:** `src/functions/hello/handler.ts`
- **Config patch (HTTP):** add `functions.hello` with `handler`, `events` (http method/path)
- **Config patch (cron):** add function with `events` (cron schedule)
- **Config patch (queue):** add function with `events` (queue name)

### 6.3 Validation

- Reuse same provider/trigger capability checks as **init**.
- If provider does not support the chosen trigger: fail early with a clear error; do not generate or patch.

---

## 7. Design principles

1. Config-aware over flag-heavy  
2. Safe patching over full regeneration  
3. Minimal scaffolding over template bloat  
4. Shared scaffold engine for **init** + **generate**  
5. Registry-based growth (generators), not giant switch statements  

---

## 8. Architecture

### 8.1 Refactor first

Extract shared logic from **init** into reusable packages before adding generate.

### 8.2 Proposed layout

```
engine/internal/cli/
  init.go
  generate.go

engine/internal/scaffold/
  registry.go
  context.go
  project.go
  function.go
  files.go
  naming.go
  templates.go

engine/internal/configpatch/
  load.go
  functions.go
  resources.go
  write.go
  diff.go
```

### 8.3 Generator abstraction

```go
type Generator interface {
    Name() string
    Kind() string
    Supports(provider string, lang string) bool
    Generate(ctx Context, input Input) ([]FileOp, []YamlPatch, error)
}
```

Registry: e.g. `function/http`, `function/cron`, `function/queue`.

### 8.4 YAML patching (critical)

- **Never** regenerate the full `runfabric.yml` to add one artifact.
- Load existing config → append/merge function definitions → preserve user content and order where possible.
- Safeguards: backup before write; no-op if patch changes nothing; collision detection for duplicate function names; validate final config before write.

---

## 9. UX

- **Non-interactive:** `runfabric generate function hello --trigger http --route GET:/hello`
- **Interactive:** `runfabric generate function` → prompt for name, trigger, route/schedule/queue, optional test file.
- **Dry-run:** Show plan (files to create, config diff) without writing.
- **Force:** Overwrite generated **files** only; never use force to overwrite config blindly.

---

## 10. File generation

- Generate only: handler file, optional test file, config patch.
- Do **not** regenerate: `package.json`, `tsconfig.json`, README, `.env.example` (that is **init**’s job).
- Keep templates minimal (e.g. `function_http.ts.tpl`, `function_cron.ts.tpl`, `function_queue.ts.tpl`).

---

## 11. Implementation order

1. **Refactor** — Extract shared scaffold logic from init into `engine/internal/scaffold` and add `engine/internal/configpatch`.  
2. **MVP** — Ship `runfabric generate function` with http/cron/queue.  
3. **Harden** — `--dry-run`, `--json`, `--force`, tests, better diffs.  
4. **Extend** — generate resource, addon, provider-override (later phases).  

---

## 12. Non-negotiable rules

1. Do not duplicate init logic inside `generate.go`.  
2. Do not regenerate the entire config file.  
3. Do not ship many artifact types in v1.  
4. Do not make generate a simple file copier.  
5. Do not let `--force` overwrite config blindly.  

---

## 13. Success criteria

- Users can add a new function without hand-editing config.  
- Generated artifacts match existing project conventions.  
- Unsupported provider/trigger combinations fail early with clear errors.  
- Config is patched safely and predictably.  
- `--dry-run` previews changes without writing files.  

---

## 14. See also

- [ROADMAP.md](ROADMAP.md) — P1 phase entry for generate  
- [COMMAND_REFERENCE.md](COMMAND_REFERENCE.md) — will list `generate` once implemented  
- [QUICKSTART.md](QUICKSTART.md) — will reference generate for adding functions  

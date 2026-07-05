# workflow-pipeline

A durable **workflow** example: a document-review service whose `runfabric.yml`
defines two workflows alongside ordinary HTTP functions. It shows every workflow
step kind and how the runtime journals, pauses, and resumes runs.

The workflow runtime is driven by `runfabric workflow run|status|cancel|replay`.
Every run is written to `.runfabric/runs/<stage>/<runId>.json`, so runs survive
the process and can be inspected or replayed later.

## Step kinds

| Kind             | Purpose                              | Required input        | Runs offline?                 |
| ---------------- | ------------------------------------ | --------------------- | ----------------------------- |
| `code`           | run application code                 | — (or `function`)     | yes (no `function`)           |
| `ai-generate`    | free-text LLM generation             | `prompt`              | no — needs an LLM endpoint     |
| `ai-structured`  | schema-constrained LLM output        | `schema`              | no — needs an LLM endpoint     |
| `ai-retrieval`   | retrieval / RAG lookup               | `query`               | no — needs a retrieval backend |
| `ai-eval`        | gate on a numeric score vs threshold | numeric `score`       | yes                           |
| `human-approval` | pause for a human decision           | — (or `approvalDecision`) | yes                       |

A `code` step with `input.function` **invokes a deployed function** (so it needs
a prior `runfabric deploy`); without it, it is a pure code node that runs
anywhere.

## Quick start — runs locally, no setup

The `quickstart-review` workflow (`code` → `ai-eval` → `human-approval`) is fully
deterministic: no cloud credentials, no LLM. From this directory:

```bash
# validate config + provider readiness
runfabric doctor

# run the workflow (auto-picks the only unambiguous name if --name is omitted)
runfabric workflow run --name quickstart-review
```

Expected: the run completes with status `ok` and all three steps `ok`. Grab the
`runId` from the output and inspect it:

```bash
runfabric workflow status --run-id <runId>
```

### Pausing for a human

The `signoff` step presets `approvalDecision: approve` so the run completes
unattended. Remove that line and the run **pauses** at `signoff`
(`status: paused`, step "awaiting human approval") until a decision is supplied —
this is how human-in-the-loop gates work.

## Full tour — `ai-review`

`ai-review` chains `ai-generate` → `ai-structured` → `ai-eval` →
`human-approval` → `code(archive)`. It needs two things the quickstart doesn't:

- **An LLM** for the `ai-generate` / `ai-structured` steps — set
  `RUNFABRIC_LLM_ENDPOINT`, or deploy to a cloud provider whose managed model is
  configured. Without it those steps fail with
  `requires an LLMClient: set RUNFABRIC_LLM_ENDPOINT ...`.
- **A prior deploy** for the final `archive` code step, which invokes the
  deployed `archive` function:

  ```bash
  runfabric deploy                       # needs AWS credentials (this config targets aws-lambda)
  runfabric workflow run --name ai-review
  ```

## Replay and cancel

```bash
# re-run from a specific step (e.g. after fixing a downstream bug)
runfabric workflow replay --run-id <runId> --step score

# request cancellation of an in-flight run
runfabric workflow cancel --run-id <runId>
```

## Files

| File             | What it is                                                             |
| ---------------- | --------------------------------------------------------------------- |
| `runfabric.yml`  | service, two workflows, and the `submit` / `archive` functions        |
| `src/submit.ts`  | HTTP entry point (`POST /documents`) — shows workflows + HTTP coexist  |
| `src/archive.ts` | code-step target invoked by the `ai-review` `archive` step (no trigger) |

## Provider

This example targets `aws-lambda`; the `quickstart-review` flow runs locally
regardless of provider. To target another cloud, change `provider.name` (see
`../hello-http/PROVIDERS.md` for per-provider config patterns).

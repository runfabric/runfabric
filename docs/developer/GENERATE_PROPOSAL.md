# Generate Command Interactive Proposal

This document captures the interactive UX behavior for `runfabric generate` in Phase 2.

## Goals

- Keep CI-friendly deterministic behavior as the default for scripted flows.
- Provide guided prompt flows for common scaffolding tasks.
- Validate inputs before writes and show a preview/confirm step.

## Behavior Summary

- `--interactive` enables prompts for missing and invalid fields.
- `--no-interactive` (or global `--non-interactive`) disables prompts.
- Using both flags together is invalid and returns an error.

## Prompt Flows

### Function

`runfabric generate function --interactive`

Prompts:

1. function name
2. language (`js|ts|python|go`)
3. trigger (`http|cron|queue`)
4. trigger-specific options (`route` or `schedule` or `queue-name`)
5. entry path
6. preview + confirm

Validation and recovery:

- Rejects empty names and path separators.
- Re-prompts unsupported trigger/provider combinations.
- Re-prompts if function key already exists.

### Resource

`runfabric generate resource --interactive`

Prompts:

1. resource name
2. resource type (`database|cache|queue`)
3. connection env var
4. preview + confirm

Validation and recovery:

- Re-prompts invalid type values.
- Re-prompts duplicate resource names.

### Addon

`runfabric generate addon --interactive`

Prompts:

1. addon name
2. addon version (optional)
3. preview + confirm

Validation and recovery:

- Re-prompts duplicate addon names.

### Provider override

`runfabric generate provider-override --interactive`

Prompts:

1. override key
2. provider name
3. runtime
4. region
5. preview + confirm

Validation and recovery:

- Re-prompts duplicate override keys.
- Re-prompts provider names that do not support the existing project trigger set per the Trigger Capability Matrix.
- Non-interactive mode fails fast when the selected provider override cannot support triggers already present in the current project.

## Non-interactive Contract

Automation-safe mode remains stable:

- explicit flags and args are required for scripted flows
- no prompt is emitted under `--no-interactive` or global `--non-interactive`
- failures remain explicit (invalid flags, missing required values)

## Test Coverage

Current coverage includes:

- interactive/non-interactive flag conflict checks
- interactive prompt flow tests for function/resource/addon
- provider-override trigger matrix validation in both interactive and non-interactive flows
- existing compatibility tests for dry-run and non-interactive behavior

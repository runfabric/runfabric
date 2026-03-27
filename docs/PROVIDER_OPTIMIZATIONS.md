# Provider Optimizations for AI Workflow Runtime

This guide describes provider-specific optimization extension points in `platform/deploy/controlplane`.

## Principle

Core AI execution remains provider-neutral. Provider behavior is injected through interfaces.

## Extension Points

- Prompt formatting: `PromptRenderer` with provider implementations in `workflow_prompt_renderers.go`.
- Tool result normalization: `ToolResultMapper` in `workflow_tool_mappers.go`.
- Model output shaping: `ModelOutputShaper` in `workflow_model_shapers.go`.
- Telemetry sink: `StepTelemetryHook` in `workflow_telemetry_hooks.go`.
- Retry behavior: `RetryStrategy` in `workflow_retry_strategies.go`.
- Model routing: `ModelSelector` in `workflow_model_selector.go`.
- Cache scoping: `CacheKeyGenerator` in `workflow_cache_keyers.go`.
- Cost accounting: `CostTracker` in `workflow_cost_trackers.go`.

## Wiring Pattern

`NewTypedStepHandlerFromConfig` detects provider from config and injects provider-specific implementations into `DefaultAIStepRunner`.

## Adding a New Provider

1. Implement provider-specific types for each interface where needed.
2. Extend `ProviderXxx(...)` selector functions.
3. Add tests in `workflow_optimizations_test.go`.
4. Keep defaults intact so unknown providers still work via default/noop implementations.

## Policy-Aware MCP Controls

Provider-level MCP policy rules are configured under `policies.mcp.providers.<provider>` and enforced in `MCPRuntime.ensureAllowed`.

Supported provider rule keys:

- `requiredRegion`
- `requiredAuth`
- `denyCrossRegion`
- `denyRegions`

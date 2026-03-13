# Comparison

> Source of truth: [README.md](../README.md) and [docs/ARCHITECTURE.md](./ARCHITECTURE.md). If this document conflicts with those files, follow those files.

## Categories

- `serverless/serverless`: original Serverless Framework project.
- `oss-serverless/serverless`: community-maintained v3-compatible fork.
- `runfabric`: multi-provider serverless CLI/deployment framework using `runfabric.yml`.

## runfabric Positioning

- `runfabric` is an alternative to Serverless Framework for multi-provider deploy workflows.
- Like Serverless Framework messaging around managed services, runfabric targets auto-scaling serverless platforms with low idle-cost overhead; its primary differentiator is one config + one CLI workflow across providers.
- It is not a drop-in config replacement for `serverless.yml`; it uses `runfabric.yml`.
- Runtime families are supported as provider capabilities allow (`nodejs|python|go|java|rust|dotnet`).
- It is not a standalone workflow orchestration runtime platform.

## Practical Selection

- Use `serverless/serverless` for mainstream Serverless Framework workflows.
- Use `oss-serverless/serverless` when you need open-source v3 continuity.
- Use `runfabric` when you want a portable provider model with unified `doctor -> plan -> build -> deploy`.

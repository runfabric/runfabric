# Extensions Addons

This directory contains local Node/TS addon implementations that follow the RunFabric Addon contract.

Addons are function/app-level build-time augmentations (for example Sentry instrumentation, generated files, env injection).

Expected module shape:

- `name`
- `kind: "addon"`
- `version`
- `supports({ runtime, provider })`
- `apply({ functionName, functionConfig, addonConfig, projectRoot, buildDir, generatedDir })`

See:

- `apps/registry/docs/ADDON_CONTRACT.md`
- `apps/registry/docs/EXTENSION_DEVELOPMENT_GUIDE.md`

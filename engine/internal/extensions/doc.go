// Package extensions implements RunFabric's dual-track extension model (Phase 15).
//
// Subpackages:
//   - extensions/providers: provider plugin contract (Provider interface, Registry, loader, errors).
//   - extensions/addons: addon contract (AddonResult, AddonApplyInput), catalog loader, validator.
//   - extensions/manifests: provider and addon manifest types and registries (list/info/search).
//
// Product language:
//   - RunFabric Extensions: umbrella term for addons and plugins.
//   - RunFabric Addons: function/app-level augmentation (Node/JS); IDs like sentry, datadog.
//   - RunFabric Plugins: Go-side providers, runtimes, simulators; IDs like aws-lambda, nodejs.
package extensions

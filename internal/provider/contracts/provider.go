package contracts

import provider "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"

// Config is the runfabric config type. Alias of the canonical definition.
type Config = provider.Config

// Result types
type DoctorResult = provider.DoctorResult
type PlanResult = provider.PlanResult
type Artifact = provider.Artifact
type DeployedFunction = provider.DeployedFunction
type DeployResult = provider.DeployResult
type RemoveResult = provider.RemoveResult
type InvokeResult = provider.InvokeResult
type LogsResult = provider.LogsResult
type OrchestrationSyncResult = provider.OrchestrationSyncResult
type MetricsResult = provider.MetricsResult
type TracesResult = provider.TracesResult

// ProviderPlugin is the canonical interface for provider plugins.
type ProviderPlugin = provider.ProviderPlugin

// ProviderRegistry is the canonical registry interface.
type ProviderRegistry = provider.ProviderRegistry

// Optional capability interfaces.
type OrchestrationCapable = provider.OrchestrationCapable
type ObservabilityCapable = provider.ObservabilityCapable
type DevStreamCapable = provider.DevStreamCapable
type RecoveryCapable = provider.RecoveryCapable

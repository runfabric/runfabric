package contracts

import provider "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"

// ProviderMeta is the plugin metadata. Alias of the canonical definition.
type ProviderMeta = provider.ProviderMeta

// Request types
type ValidateConfigRequest = provider.ValidateConfigRequest
type DoctorRequest = provider.DoctorRequest
type PlanRequest = provider.PlanRequest
type DeployRequest = provider.DeployRequest
type RemoveRequest = provider.RemoveRequest
type InvokeRequest = provider.InvokeRequest
type LogsRequest = provider.LogsRequest

// Orchestration requests
type OrchestrationSyncRequest = provider.OrchestrationSyncRequest
type OrchestrationRemoveRequest = provider.OrchestrationRemoveRequest
type OrchestrationInvokeRequest = provider.OrchestrationInvokeRequest
type OrchestrationInspectRequest = provider.OrchestrationInspectRequest

// Observability requests
type MetricsRequest = provider.MetricsRequest
type TracesRequest = provider.TracesRequest

// DevStream types
type DevStreamRequest = provider.DevStreamRequest
type DevStreamSession = provider.DevStreamSession

// Recovery types
type RecoveryRequest = provider.RecoveryRequest
type RecoveryResult = provider.RecoveryResult

// NewDevStreamSession is a convenience constructor forwarded from the core package.
var NewDevStreamSession = provider.NewDevStreamSession

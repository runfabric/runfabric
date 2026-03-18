// Package providers re-exports the provider contract and registry from extensions/providers
// so that engine/internal/extensions/provider/* and other packages can keep importing engine/internal/providers.
// The canonical implementation lives in engine/internal/extensions/providers.
package providers

import (
	ext "github.com/runfabric/runfabric/engine/internal/extensions/providers"
)

type (
	Config       = ext.Config
	DoctorResult = ext.DoctorResult
	PlanResult   = ext.PlanResult
	Artifact     = ext.Artifact
	DeployResult = ext.DeployResult
	RemoveResult = ext.RemoveResult
	InvokeResult = ext.InvokeResult
	LogsResult   = ext.LogsResult
	Provider     = ext.Provider
	Registry     = ext.Registry
	// Recommended plugin interface and metadata (context + request/result).
	ProviderMeta          = ext.ProviderMeta
	ProviderPlugin        = ext.ProviderPlugin
	ProviderRegistry      = ext.ProviderRegistry
	ValidateConfigRequest = ext.ValidateConfigRequest
	DoctorRequest         = ext.DoctorRequest
	PlanRequest           = ext.PlanRequest
	DeployRequest         = ext.DeployRequest
	RemoveRequest         = ext.RemoveRequest
	InvokeRequest         = ext.InvokeRequest
	LogsRequest           = ext.LogsRequest
)

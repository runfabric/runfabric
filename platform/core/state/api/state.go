package api

import internalstate "github.com/runfabric/runfabric/platform/core/state/core"

// Receipt is a public wrapper over platform/core/state/core.Receipt so extension
// providers can read/write deployment receipts without importing state internals.
type Receipt = internalstate.Receipt

// FunctionDeployment is a public wrapper over platform/core/state/core.FunctionDeployment.
type FunctionDeployment = internalstate.FunctionDeployment

const CurrentReceiptVersion = internalstate.CurrentReceiptVersion

// Save writes the deployment receipt for the given stage under root/.runfabric.
func Save(root string, receipt *Receipt) error { return internalstate.Save(root, receipt) }

// Load reads the deployment receipt for the given stage from root/.runfabric.
func Load(root, stage string) (*Receipt, error) { return internalstate.Load(root, stage) }

// Delete removes the deployment receipt for the given stage.
func Delete(root, stage string) error { return internalstate.Delete(root, stage) }

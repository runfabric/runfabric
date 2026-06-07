package core

import "fmt"

// MigrateReceipt upgrades a receipt loaded from disk to the current schema.
//
// The receipt schema is additive: every field added across versions is
// omitempty, so an older receipt (including a pre-versioning receipt whose
// Version field is absent and unmarshals to 0) is forward-compatible — newer
// fields simply read as their zero values. Such receipts are accepted and
// stamped to the current version. A receipt newer than this binary understands
// cannot be safely downgraded and is rejected.
//
// If a future schema change is non-additive, add the field-level transformation
// here, keyed on in.Version, before stamping CurrentReceiptVersion.
func MigrateReceipt(in *Receipt) (*Receipt, error) {
	if in == nil {
		return nil, fmt.Errorf("nil receipt")
	}
	if in.Version > CurrentReceiptVersion {
		return nil, fmt.Errorf("receipt version %d is newer than supported version %d; upgrade runfabric", in.Version, CurrentReceiptVersion)
	}
	in.Version = CurrentReceiptVersion
	return in, nil
}

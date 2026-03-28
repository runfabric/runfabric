package core

import "fmt"

func MigrateReceipt(in *Receipt) (*Receipt, error) {
	if in == nil {
		return nil, fmt.Errorf("nil receipt")
	}

	switch in.Version {
	case CurrentReceiptVersion:
		return in, nil
	default:
		return nil, fmt.Errorf("unsupported receipt version %d", in.Version)
	}
}

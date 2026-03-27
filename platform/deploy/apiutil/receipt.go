package apiutil

import "encoding/json"

// ReceiptView is a minimal, transport-safe view of state receipt fields used by providers.
type ReceiptView struct {
	Outputs  map[string]string `json:"outputs"`
	Metadata map[string]string `json:"metadata"`
}

// DecodeReceipt converts an arbitrary receipt payload into a stable map-based view.
func DecodeReceipt(receipt any) ReceiptView {
	view := ReceiptView{
		Outputs:  map[string]string{},
		Metadata: map[string]string{},
	}
	if receipt == nil {
		return view
	}
	b, err := json.Marshal(receipt)
	if err != nil {
		return view
	}
	_ = json.Unmarshal(b, &view)
	if view.Outputs == nil {
		view.Outputs = map[string]string{}
	}
	if view.Metadata == nil {
		view.Metadata = map[string]string{}
	}
	return view
}

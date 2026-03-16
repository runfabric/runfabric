package protocol

type InspectResult struct {
	Service string      `json:"service"`
	Stage   string      `json:"stage"`
	Lock    interface{} `json:"lock,omitempty"`
	Journal interface{} `json:"journal,omitempty"`
	Receipt interface{} `json:"receipt,omitempty"`
}

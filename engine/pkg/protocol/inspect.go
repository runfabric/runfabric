package protocol

type InspectResult struct {
	Service    string      `json:"service"`
	Stage      string      `json:"stage"`
	Lock       interface{} `json:"lock,omitempty"`
	Journal    interface{} `json:"journal,omitempty"`
	Receipt    interface{} `json:"receipt,omitempty"`
	AiWorkflow interface{} `json:"aiWorkflow,omitempty"` // Phase 14.6: hash, entrypoint, cost summary when aiWorkflow enabled
}

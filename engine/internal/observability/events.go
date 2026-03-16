package observability

type Event struct {
	Type      string            `json:"type"`
	Service   string            `json:"service"`
	Stage     string            `json:"stage"`
	Message   string            `json:"message,omitempty"`
	Timestamp string            `json:"timestamp"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

package wrapper

type Handshake struct {
	Version           string   `json:"version"`
	ProtocolVersion   string   `json:"protocolVersion"`
	Platform          string   `json:"platform"`
	Capabilities      []string `json:"capabilities,omitempty"`
	SupportsRuntime   []string `json:"supportsRuntime,omitempty"`
	SupportsTriggers  []string `json:"supportsTriggers,omitempty"`
	SupportsResources []string `json:"supportsResources,omitempty"`
}

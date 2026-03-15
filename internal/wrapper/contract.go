package wrapper

type Handshake struct {
	Version         string `json:"version"`
	ProtocolVersion string `json:"protocolVersion"`
	Platform        string `json:"platform"`
}

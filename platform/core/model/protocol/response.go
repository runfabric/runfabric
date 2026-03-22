package protocol

type Response struct {
	OK      bool     `json:"ok"`
	Command string   `json:"command"`
	Data    any      `json:"data,omitempty"`
	Error   *ErrBody `json:"error,omitempty"`
}

type ErrBody struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

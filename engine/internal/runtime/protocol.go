package runtime

type Response struct {
	OK      bool           `json:"ok"`
	Command string         `json:"command"`
	Result  map[string]any `json:"result,omitempty"`
	Error   *ErrorResponse `json:"error,omitempty"`
}

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

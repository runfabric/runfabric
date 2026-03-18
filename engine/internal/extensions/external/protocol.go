package external

// Protocol is line-delimited JSON over stdio.
// One request JSON object per line, one response JSON object per line.

type Request struct {
	ID              string `json:"id"`
	Method          string `json:"method"`
	ProtocolVersion string `json:"protocolVersion,omitempty"`
	Params          any    `json:"params,omitempty"`
}

type Response struct {
	ID     string         `json:"id"`
	Result any            `json:"result,omitempty"`
	Error  *ResponseError `json:"error,omitempty"`
}

type ResponseError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

package protocol

import "encoding/json"

// Request is one line-delimited JSON request sent by RunFabric engine.
type Request struct {
	ID              string          `json:"id"`
	Method          string          `json:"method"`
	ProtocolVersion string          `json:"protocolVersion,omitempty"`
	Params          json.RawMessage `json:"params,omitempty"`
}

// Response is one line-delimited JSON response returned by a plugin.
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

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

// Event is a streaming message pushed by a plugin before the final Response.
// Distinguished from Response by the presence of "type" and absence of "id".
//
// Wire format (newline-delimited JSON, same transport as Request/Response):
//
//	{"type":"progress","requestId":"req-1","message":"Building image..."}
//	{"type":"log","requestId":"req-1","level":"info","line":"handler loaded","timestamp":"..."}
//	{"type":"warn","requestId":"req-1","message":"no GHCR_TOKEN set"}
//	{"id":"req-1","result":{...}}   ← final Response
type Event struct {
	Type      string `json:"type"`                // progress | log | warn
	RequestID string `json:"requestId,omitempty"` // mirrors the originating request ID
	Message   string `json:"message,omitempty"`   // for progress / warn
	Line      string `json:"line,omitempty"`      // for log
	Level     string `json:"level,omitempty"`     // for log: debug | info | warn | error
	Timestamp string `json:"timestamp,omitempty"` // RFC3339
}

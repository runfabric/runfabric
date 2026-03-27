package configapi

import "encoding/json"

// ResolveResponse is the daemon-owned DTO for resolved config responses.
type ResolveResponse struct {
	Payload json.RawMessage
}

// PlanResponse is the daemon-owned DTO for plan responses.
type PlanResponse struct {
	Payload json.RawMessage
}

// DeployResponse is the daemon-owned DTO for deploy responses.
type DeployResponse struct {
	Payload json.RawMessage
}

// RemoveResponse is the daemon-owned DTO for remove responses.
type RemoveResponse struct {
	Payload json.RawMessage
}

// ReleasesResponse is the daemon-owned DTO for releases responses.
type ReleasesResponse struct {
	Payload json.RawMessage
}

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

type request struct {
	ID              string         `json:"id"`
	Method          string         `json:"method"`
	ProtocolVersion string         `json:"protocolVersion,omitempty"`
	Params          map[string]any `json:"params,omitempty"`
}

type response struct {
	ID     string         `json:"id"`
	Result map[string]any `json:"result,omitempty"`
	Error  *respErr       `json:"error,omitempty"`
}

type respErr struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)
	for scanner.Scan() {
		line := scanner.Bytes()
		var req request
		if err := json.Unmarshal(line, &req); err != nil {
			_ = encoder.Encode(response{
				ID: req.ID,
				Error: &respErr{
					Code:    "bad_request",
					Message: fmt.Sprintf("invalid json: %v", err),
				},
			})
			continue
		}
		_ = encoder.Encode(handle(req))
	}
}

func handle(req request) response {
	switch req.Method {
	case "Handshake":
		return response{
			ID: req.ID,
			Result: map[string]any{
				"protocolVersion": "1",
				"capabilities":    []string{"Sync"},
			},
		}
	case "Sync":
		return response{
			ID: req.ID,
			Result: map[string]any{
				"dryRun": true,
				"actions": []map[string]any{
					{
						"resource": "dns_record_set",
						"action":   "plan",
						"name":     "example.com",
						"detail":   "replace this with provider-specific sync logic",
					},
				},
			},
		}
	default:
		return response{
			ID: req.ID,
			Error: &respErr{
				Code:    "unsupported_method",
				Message: "method not implemented: " + req.Method,
			},
		}
	}
}

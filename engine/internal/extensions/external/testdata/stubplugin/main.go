package main

import (
	"bufio"
	"encoding/json"
	"os"
	"runtime"

	extRuntime "github.com/runfabric/runfabric/engine/internal/extensions/runtime"
	"github.com/runfabric/runfabric/engine/internal/wrapper"
)

type Request struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	ID     string     `json:"id"`
	Result any        `json:"result,omitempty"`
	Error  *RespError `json:"error,omitempty"`
}

type RespError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}

func main() {
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		var req Request
		if err := json.Unmarshal(sc.Bytes(), &req); err != nil {
			_ = json.NewEncoder(os.Stdout).Encode(Response{ID: "?", Error: &RespError{Code: "bad_json", Message: err.Error()}})
			continue
		}
		switch req.Method {
		case "Handshake":
			_ = json.NewEncoder(os.Stdout).Encode(Response{
				ID: req.ID,
				Result: wrapper.Handshake{
					Version:         "stub",
					ProtocolVersion: extRuntime.ProtocolVersion,
					Platform:        runtime.GOOS + "-" + runtime.GOARCH,
				},
			})
		case "Doctor":
			_ = json.NewEncoder(os.Stdout).Encode(Response{ID: req.ID, Result: map[string]any{"provider": "stub", "checks": []string{"ok"}}})
		case "Invoke":
			_ = json.NewEncoder(os.Stdout).Encode(Response{ID: req.ID, Result: map[string]any{"provider": "stub", "function": "fn", "output": "pong"}})
		default:
			_ = json.NewEncoder(os.Stdout).Encode(Response{ID: req.ID, Result: map[string]any{"provider": "stub"}})
		}
	}
	// Scanner exits when stdin is closed.
}

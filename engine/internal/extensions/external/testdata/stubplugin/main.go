package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
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
	if !sc.Scan() {
		return
	}
	var req Request
	if err := json.Unmarshal(sc.Bytes(), &req); err != nil {
		_ = json.NewEncoder(os.Stdout).Encode(Response{ID: "?", Error: &RespError{Code: "bad_json", Message: err.Error()}})
		return
	}
	switch req.Method {
	case "Doctor":
		_ = json.NewEncoder(os.Stdout).Encode(Response{ID: req.ID, Result: map[string]any{"provider": "stub", "checks": []string{"ok"}}})
	case "Invoke":
		_ = json.NewEncoder(os.Stdout).Encode(Response{ID: req.ID, Result: map[string]any{"provider": "stub", "function": "fn", "output": "pong"}})
	default:
		_ = json.NewEncoder(os.Stdout).Encode(Response{ID: req.ID, Result: map[string]any{"provider": "stub"}})
	}
	fmt.Fprintln(os.Stderr, "")
}

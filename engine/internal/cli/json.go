package cli

import (
	"encoding/json"
	"io"
	"os"

	"github.com/runfabric/runfabric/engine/internal/runtime"
)

// JSONEnvelope is the standard --json result shape for dashboard tooling (Phase 13.11.4).
type JSONEnvelope struct {
	OK      bool                   `json:"ok"`
	Command string                 `json:"command"`
	Result  map[string]any         `json:"result,omitempty"`
	Error   *runtime.ErrorResponse `json:"error,omitempty"`
}

// WriteJSONEnvelope writes the standard JSON envelope to w. Prefer passing cmd.OutOrStdout() from RunE so output is testable and respects --output.
func WriteJSONEnvelope(w io.Writer, ok bool, command string, result map[string]any, errResp *runtime.ErrorResponse) error {
	out := JSONEnvelope{OK: ok, Command: command, Result: result, Error: errResp}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func printJSONSuccess(command string, result any) error {
	return WriteJSONEnvelope(os.Stdout, true, command, toMap(result), nil)
}

// printJSONFailure emits machine-readable error output for --json. Use when opts.JSONOutput and err != nil for consistent JSON shape.
func printJSONFailure(command string, code string, err error) error {
	return WriteJSONEnvelope(os.Stdout, false, command, nil, &runtime.ErrorResponse{Code: code, Message: err.Error()})
}

func toMap(v any) map[string]any {
	b, _ := json.Marshal(v)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m
}

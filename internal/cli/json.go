package cli

import (
	"encoding/json"
	"os"

	"github.com/runfabric/runfabric/internal/runtime"
)

func printJSONSuccess(command string, result any) error {
	out := runtime.Response{
		OK:      true,
		Command: command,
		Result:  toMap(result),
	}
	enc := json.NewEncoder(os.Stdout)
	return enc.Encode(out)
}

func printJSONFailure(command string, code string, err error) error {
	out := runtime.Response{
		OK:      false,
		Command: command,
		Error: &runtime.ErrorResponse{
			Code:    code,
			Message: err.Error(),
		},
	}
	enc := json.NewEncoder(os.Stdout)
	return enc.Encode(out)
}

func toMap(v any) map[string]any {
	b, _ := json.Marshal(v)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m
}

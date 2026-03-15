package cli

import (
	"encoding/json"
	"fmt"

	"github.com/runfabric/runfabric/pkg/protocol"
)

func printSuccess(command string, data any) error {
	resp := protocol.Success(command, data)
	return printJSON(resp)
}

func printFailure(command string, err error) error {
	resp := protocol.Failure(command, protocol.FromError(err))
	return printJSON(resp)
}

func printJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal output: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

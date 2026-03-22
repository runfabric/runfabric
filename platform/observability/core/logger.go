package core

import (
	"encoding/json"
	"os"
)

func Emit(event *Event) error {
	enc := json.NewEncoder(os.Stdout)
	return enc.Encode(event)
}

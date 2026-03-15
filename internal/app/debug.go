package app

import (
	"fmt"
	"os"
)

// Debug runs the service locally and prints instructions to attach a debugger.
// Same as CallLocal(..., serve=true) but also prints process PID and debug tip.
func Debug(configPath, stage, host, port string) (any, error) {
	fmt.Fprintf(os.Stderr, "Debug: attach your debugger to this process (PID %d). Listening on %s:%s\n", os.Getpid(), host, port)
	return CallLocal(configPath, stage, host, port, true)
}

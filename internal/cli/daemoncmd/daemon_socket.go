//go:build !windows

package daemoncmd

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
)

const daemonSocketName = "daemon.sock"

// listenOnSocket binds handler to a Unix domain socket at .runfabric/daemon.sock
// alongside the TCP listener. Stale sockets are removed before binding.
// Returns the socket path, or empty string with a logged warning on failure.
func listenOnSocket(handler http.Handler) string {
	dir, err := daemonDir()
	if err != nil {
		return ""
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ""
	}
	sockPath := filepath.Join(dir, daemonSocketName)
	_ = os.Remove(sockPath) // remove stale socket from a previous run

	l, err := net.Listen("unix", sockPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not bind unix socket %s: %v\n", sockPath, err)
		return ""
	}
	if err := os.Chmod(sockPath, 0o600); err != nil {
		_ = l.Close()
		return ""
	}
	go func() {
		srv := &http.Server{Handler: handler}
		_ = srv.Serve(l)
		_ = os.Remove(sockPath)
	}()
	return sockPath
}

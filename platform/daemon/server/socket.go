//go:build !windows

package server

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
)

const socketName = "daemon.sock"

// ListenOnSocket binds handler to a Unix domain socket at <dir>/daemon.sock.
// dir should be the .runfabric directory (e.g. filepath.Join(cwd, ".runfabric")).
// Stale sockets are removed before binding. Returns the socket path on success,
// or empty string with a warning printed to stderr on failure.
func ListenOnSocket(handler http.Handler) string {
	dir, err := defaultSocketDir()
	if err != nil {
		return ""
	}
	return ListenOnSocketAt(dir, handler)
}

// ListenOnSocketAt binds to <dir>/daemon.sock. Exported for callers that
// know the exact .runfabric directory (e.g. non-default workspace).
func ListenOnSocketAt(dir string, handler http.Handler) string {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ""
	}
	sockPath := filepath.Join(dir, socketName)
	_ = os.Remove(sockPath)

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

func defaultSocketDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, ".runfabric"), nil
}

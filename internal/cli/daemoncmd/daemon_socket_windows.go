//go:build windows

package daemoncmd

import "net/http"

// listenOnSocket is a no-op on Windows; Unix domain sockets are not used.
func listenOnSocket(_ http.Handler) string { return "" }

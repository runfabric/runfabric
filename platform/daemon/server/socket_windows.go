//go:build windows

package server

import "net/http"

// ListenOnSocket is a no-op on Windows; Unix domain sockets are not used.
func ListenOnSocket(_ http.Handler) string { return "" }

// ListenOnSocketAt is a no-op on Windows.
func ListenOnSocketAt(_ string, _ http.Handler) string { return "" }

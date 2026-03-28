package protocol

import (
	"runtime"

	rt "github.com/runfabric/runfabric/platform/extensions/registry/loader/runtime"
)

func CurrentHandshake() Handshake {
	return Handshake{
		Version:         rt.Version,
		ProtocolVersion: rt.ProtocolVersion,
		Platform:        runtime.GOOS + "-" + runtime.GOARCH,
	}
}

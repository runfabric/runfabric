package wrapper

import (
	"runtime"

	rt "github.com/runfabric/runfabric/engine/internal/extensions/runtime"
)

func CurrentHandshake() Handshake {
	return Handshake{
		Version:         rt.Version,
		ProtocolVersion: rt.ProtocolVersion,
		Platform:        runtime.GOOS + "-" + runtime.GOARCH,
	}
}

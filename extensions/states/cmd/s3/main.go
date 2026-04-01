package main

import (
	"context"
	"fmt"
	"os"

	"github.com/runfabric/runfabric/extensions/states"
	sdkstate "github.com/runfabric/runfabric/plugin-sdk/go/state"
)

func main() {
	plugin, err := states.NewS3TransportPlugin()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	server := sdkstate.NewServer(plugin, sdkstate.ServeOptions{ProtocolVersion: "1"})
	if err := server.Serve(context.Background(), os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

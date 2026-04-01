package main

import (
	"context"
	"fmt"
	"os"

	plugin "github.com/runfabric/runfabric/extensions/routers/azuretrafficmanager"
	sdkrouter "github.com/runfabric/runfabric/plugin-sdk/go/router"
)

func main() {
	p := plugin.NewTransportPlugin()
	s := sdkrouter.NewServer(p, sdkrouter.ServeOptions{ProtocolVersion: "1"})
	if err := s.Serve(context.Background(), os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

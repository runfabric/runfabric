package main

import (
	"context"
	"fmt"
	"os"

	kubernetesprovider "github.com/runfabric/runfabric/extensions/providers/kubernetes"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

func main() {
	plugin := kubernetesprovider.NewTransportPlugin()
	server := sdkprovider.NewServer(plugin, sdkprovider.ServeOptions{ProtocolVersion: "1"})
	if err := server.Serve(context.Background(), os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

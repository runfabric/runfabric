package main

import (
	"context"
	"fmt"
	"os"

	cloudflareprovider "github.com/runfabric/runfabric/extensions/providers/cloudflare"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

func main() {
	plugin := cloudflareprovider.NewTransportPlugin()
	server := sdkprovider.NewServer(plugin, sdkprovider.ServeOptions{ProtocolVersion: "1"})
	if err := server.Serve(context.Background(), os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

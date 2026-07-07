// Command linode is the RunFabric provider plugin for Linode. It advertises
// HTTP-triggered nodejs/python functions and drives deploy/remove/invoke/logs
// through configurable shell commands, with an HTTP fast-path for invocation.
package main

import (
	"context"
	"fmt"
	"os"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

func main() {
	p := newPlugin()
	s := sdkprovider.NewServer(p, sdkprovider.ServeOptions{ProtocolVersion: "1"})
	if err := s.Serve(context.Background(), os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

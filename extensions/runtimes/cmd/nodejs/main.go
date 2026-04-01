package main

import (
	"context"
	"fmt"
	"os"

	"github.com/runfabric/runfabric/extensions/runtimes"
	sdkruntime "github.com/runfabric/runfabric/plugin-sdk/go/runtime"
)

func main() {
	p := runtimes.NewNodeRuntime()
	s := sdkruntime.NewServer(p, sdkruntime.ServeOptions{ProtocolVersion: "1"})
	if err := s.Serve(context.Background(), os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

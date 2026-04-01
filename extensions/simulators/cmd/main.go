package main

import (
	"context"
	"fmt"
	"os"

	"github.com/runfabric/runfabric/extensions/simulators"
	sdksimulator "github.com/runfabric/runfabric/plugin-sdk/go/simulator"
)

func main() {
	p := simulators.NewLocalSimulator()
	s := sdksimulator.NewServer(p, sdksimulator.ServeOptions{ProtocolVersion: "1"})
	if err := s.Serve(context.Background(), os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

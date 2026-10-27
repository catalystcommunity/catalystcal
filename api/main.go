package main

import (
	"fmt"
	"os"

	"github.com/catalystcommunity/catalystcal/api/cmd"
)

// Version is set at link time: -ldflags "-X main.Version=1.2.3".
var Version = "dev"

func main() {
	if err := cmd.Run(Version, os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

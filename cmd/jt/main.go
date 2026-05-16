package main

import (
	"fmt"
	"os"

	"github.com/okaris/jsont/pkg/cli"
)

var version = "0.1.0" // overridden by -ldflags at build time

func main() {
	cli.Version = version
	if err := cli.Run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "jt: %s\n", err)
		os.Exit(1)
	}
}

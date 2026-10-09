// Command forge is the developer CLI for the versioned registration API.
package main

import (
	"os"

	"github.com/octieght18/forge/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, nil))
}

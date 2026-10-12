package main

import (
	"os"

	"github.com/octieght18/forge/internal/mcp"
)

func main() {
	if err := mcp.Serve(os.Stdin, os.Stdout); err != nil {
		os.Exit(1)
	}
}

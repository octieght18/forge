// forge-promote copies the local release pin to staging.
// It refuses a cloud destination and does not connect to PostgreSQL or Temporal.
package main

import (
	"fmt"
	"os"

	"github.com/octieght18/forge/internal/delivery"
)

func run(args []string) error {
	root := ""
	from := ""
	to := ""
	confirm := ""
	for i := 0; i < len(args); i++ {
		if i+1 >= len(args) {
			return fmt.Errorf("argument requires a value")
		}
		switch args[i] {
		case "--overlays":
			i++
			root = args[i]
		case "--from":
			i++
			from = args[i]
		case "--to":
			i++
			to = args[i]
		case "--confirm":
			i++
			confirm = args[i]
		default:
			return fmt.Errorf("unknown argument")
		}
	}
	pin, err := delivery.Promote(root, from, to, confirm)
	if err != nil {
		return err
	}
	fmt.Printf("staging pin %s\n", pin)
	return nil
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

// forge-demo-cleanup removes synthetic demo files from an explicit directory.
// It does not connect to PostgreSQL and it does not read secret files.
package main

import (
	"fmt"
	"os"

	"github.com/octieght18/forge/internal/retention"
)

func run(args []string) error {
	dir := ""
	confirm := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dir":
			i++
			if i >= len(args) {
				return fmt.Errorf("cleanup directory is required")
			}
			dir = args[i]
		case "--confirm":
			i++
			if i >= len(args) {
				return fmt.Errorf("confirmation is required")
			}
			confirm = args[i]
		default:
			return fmt.Errorf("unknown argument")
		}
	}
	removed, err := retention.Cleanup(dir, confirm)
	if err != nil {
		return err
	}
	fmt.Printf("removed %d\n", len(removed))
	for _, name := range removed {
		fmt.Println(name)
	}
	return nil
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

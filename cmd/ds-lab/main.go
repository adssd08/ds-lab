// Command ds-lab runs the introductory CLI output check.
package main

import (
	"fmt"
	"os"

	"ds-lab/internal/cli"
)

// main connects the testable CLI function to process output and exit status.
func main() {
	if err := cli.Run(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

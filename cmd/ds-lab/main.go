package main

import (
	"fmt"
	"os"

	"ds-lab/internal/cli"
)

func main() {
	if err := cli.Run(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

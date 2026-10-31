package main

import (
	"ds-lab/internal/protocol"
	"fmt"
	"os"
)

func main() {
	if err := protocol.Run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// Package cli contains the introductory CLI behavior without process-specific setup.
package cli

import (
	"fmt"
	"io"
)

// Run writes the readiness message and newline to out, returning any write error.
// Accepting a writer lets main use stdout and tests use an in-memory buffer.
func Run(out io.Writer) error {
	_, err := fmt.Fprintln(out, "ds-lab ready")
	return err
}

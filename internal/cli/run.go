package cli

import (
	"fmt"
	"io"
)

func Run(out io.Writer) error {
	_, err := fmt.Fprintln(out, "ds-lab ready")
	return err
}

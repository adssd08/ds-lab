package cli

import (
	"bytes"
	"testing"
)

// TestRun checks successful execution and exact output, including the newline.
// A buffer captures output without redirecting the process-wide stdout stream.
func TestRun(t *testing.T) {
	var out bytes.Buffer
	if err := Run(&out); err != nil {
		t.Fatalf("Run returned an error: %v", err)
	}

	if got, want := out.String(), "ds-lab ready\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

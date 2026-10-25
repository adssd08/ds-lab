package cli

import (
	"bytes"
	"testing"
)

func TestRun(t *testing.T) {
	var out bytes.Buffer
	if err := Run(&out); err != nil {
		t.Fatalf("Run returned an error: %v", err)
	}

	if got, want := out.String(), "ds-lab ready\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

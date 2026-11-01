package protocol

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

const maxInputBuffer = 1 << 20

// Run processes one JSON request per line until EOF or a Go error.
// Initialization is required; protocol error responses do not stop the stream.
// The caller owns the reader and writer; Run does not close them.
func Run(in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 4096), maxInputBuffer)

	encoder := json.NewEncoder(out)
	node := NewNode()
	var nextID uint64 = 1

	for scanner.Scan() {
		var request Envelope
		if err := json.Unmarshal(scanner.Bytes(), &request); err != nil {
			return fmt.Errorf("decode envelope: %w", err)
		}

		if nextID == 0 {
			return fmt.Errorf("response message IDs exhausted")
		}

		response, err := node.Handle(request, nextID)
		if err != nil {
			return err
		}

		if err := encoder.Encode(response); err != nil {
			return fmt.Errorf("write response: %w", err)
		}

		nextID++
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read request: %w", err)
	}

	return nil
}

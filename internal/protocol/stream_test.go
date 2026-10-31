package protocol

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"testing"
)

// decodeOne compares JSON meaning without depending on object-key order.
// UseNumber preserves integer text instead of converting numbers to float64.
func decodeOne(t *testing.T, data []byte) any {
	t.Helper()

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()

	var value any
	if err := decoder.Decode(&value); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		t.Fatalf("expected exactly one JSON value, got error %v", err)
	}

	return value
}

func TestRunEchoGolden(t *testing.T) {
	request, err := os.ReadFile("testdata/echo-request.json")

	if err != nil {
		t.Fatal(err)
	}

	expected, err := os.ReadFile("testdata/echo-response.json")
	if err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if err := Run(bytes.NewReader(request), &output); err != nil {
		t.Fatal(err)
	}

	if !bytes.HasSuffix(output.Bytes(), []byte("\n")) {
		t.Fatal("response is missing its newline")
	}

	got := decodeOne(t, output.Bytes())
	want := decodeOne(t, expected)

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("response = %s, want %s", output.Bytes(), expected)
	}
}

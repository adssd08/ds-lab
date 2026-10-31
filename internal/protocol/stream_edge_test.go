package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
)

func echoInput(t *testing.T, id uint64, text string) string {
	t.Helper()
	body, err := json.Marshal(EchoRequest{Type: "echo", MsgID: id, Echo: text})
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(Envelope{Src: "c1", Dest: "n1", Body: body})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func readReplies(t *testing.T, data []byte) []struct {
	Src  string       `json:"src"`
	Dest string       `json:"dest"`
	Body EchoResponse `json:"body"`
} {
	t.Helper()
	var result []struct {
		Src  string       `json:"src"`
		Dest string       `json:"dest"`
		Body EchoResponse `json:"body"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	for {
		var reply struct {
			Src  string       `json:"src"`
			Dest string       `json:"dest"`
			Body EchoResponse `json:"body"`
		}
		err := decoder.Decode(&reply)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		result = append(result, reply)
	}
	return result
}

func TestRunEmptyInput(t *testing.T) {
	var out bytes.Buffer
	if err := Run(strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Fatalf("unexpected output %q", out.String())
	}
}

func TestRunMultipleRequestsAndLargeIDs(t *testing.T) {
	ids := []uint64{9007199254740993, ^uint64(0)}
	input := echoInput(t, ids[0], "first") + "\n" + echoInput(t, ids[1], "second") + "\n"
	var out bytes.Buffer
	if err := Run(strings.NewReader(input), &out); err != nil {
		t.Fatal(err)
	}
	replies := readReplies(t, out.Bytes())
	if len(replies) != 2 || bytes.Count(out.Bytes(), []byte("\n")) != 2 {
		t.Fatalf("expected two response lines: %q", out.String())
	}
	for i, reply := range replies {
		wantText := []string{"first", "second"}[i]
		want := EchoResponse{Type: "echo_ok", MsgID: uint64(i + 1), InReplyTo: ids[i], Echo: wantText}
		if reply.Src != "n1" || reply.Dest != "c1" || reply.Body != want {
			t.Fatalf("reply %d = %#v, want body %#v", i, reply, want)
		}
	}
}

func TestRunPreservesTextAndFraming(t *testing.T) {
	for _, text := range []string{"", "quotes: \" backslash: \\ newline:\nUnicode: नमस्ते 🌍", strings.Repeat("x", 70*1024)} {
		for _, ending := range []string{"", "\n", "\r\n"} {
			var out bytes.Buffer
			if err := Run(strings.NewReader(echoInput(t, 17, text)+ending), &out); err != nil {
				t.Fatal(err)
			}
			replies := readReplies(t, out.Bytes())
			if len(replies) != 1 || replies[0].Body.Echo != text || replies[0].Body.InReplyTo != 17 {
				t.Fatal("text or correlation changed")
			}
			if bytes.Count(out.Bytes(), []byte("\n")) != 1 || !bytes.HasSuffix(out.Bytes(), []byte("\n")) {
				t.Fatal("response must occupy one terminated line")
			}
		}
	}
}

func TestRunRejectsInvalidRequests(t *testing.T) {
	tests := map[string]string{
		"malformed JSON":          "{",
		"blank line":              "\n",
		"two objects on one line": echoInput(t, 1, "a") + echoInput(t, 2, "b"),
		"missing source":          `{"dest":"n1","body":{"type":"echo"}}`,
		"missing destination":     `{"src":"c1","body":{"type":"echo"}}`,
		"wrong body type":         `{"src":"c1","dest":"n1","body":[]}`,
		"unsupported operation":   `{"src":"c1","dest":"n1","body":{"type":"unknown"}}`,
		"nonstring echo":          `{"src":"c1","dest":"n1","body":{"type":"echo","echo":42}}`,
		"oversized line":          echoInput(t, 1, strings.Repeat("x", maxInputBuffer)),
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			if err := Run(strings.NewReader(input), &out); err == nil {
				t.Fatal("invalid request accepted")
			}
			if out.Len() != 0 {
				t.Fatalf("invalid request produced output %q", out.String())
			}
		})
	}
}

func TestRunStopsAtFirstBadRequest(t *testing.T) {
	input := echoInput(t, 41, "before") + "\n{\n" + echoInput(t, 42, "after") + "\n"
	var out bytes.Buffer
	if err := Run(strings.NewReader(input), &out); err == nil {
		t.Fatal("expected malformed-input error")
	}
	replies := readReplies(t, out.Bytes())
	if len(replies) != 1 || replies[0].Body.Echo != "before" {
		t.Fatalf("unexpected responses: %#v", replies)
	}
}

type failingReader struct{ err error }

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }

type failingWriter struct {
	err   error
	calls int
}

func (w *failingWriter) Write([]byte) (int, error) { w.calls++; return 0, w.err }

func TestRunPropagatesReadError(t *testing.T) {
	failure := errors.New("input unavailable")
	var out bytes.Buffer
	if err := Run(failingReader{failure}, &out); !errors.Is(err, failure) {
		t.Fatalf("error = %v, want read failure", err)
	}
	if out.Len() != 0 {
		t.Fatal("read failure produced output")
	}
}

func TestRunPropagatesWriteErrorAndStops(t *testing.T) {
	failure := errors.New("output unavailable")
	writer := &failingWriter{err: failure}
	input := echoInput(t, 1, "first") + "\n" + echoInput(t, 2, "second") + "\n"
	if err := Run(strings.NewReader(input), writer); !errors.Is(err, failure) {
		t.Fatalf("error = %v, want write failure", err)
	}
	if writer.calls != 1 {
		t.Fatalf("write attempts = %d, want 1", writer.calls)
	}
}

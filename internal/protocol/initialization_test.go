package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

func initLine(t *testing.T) string {
	t.Helper()
	data, err := json.Marshal(initEnvelope(t, 10))
	if err != nil {
		t.Fatal(err)
	}
	return string(data) + "\n"
}

func TestRunInitRepeatAndEcho(t *testing.T) {
	input := initLine(t) + initLine(t) + echoInput(t, 41, "hello") + "\n"
	var out bytes.Buffer
	if err := Run(strings.NewReader(input), &out); err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(&out)
	for i, wantType := range []string{"init_ok", "error", "echo_ok"} {
		var env Envelope
		if err := decoder.Decode(&env); err != nil {
			t.Fatal(err)
		}
		if env.Src != "n1" || env.Dest != "c1" {
			t.Fatalf("wrong routing: %#v", env)
		}
		var body struct {
			Type      string `json:"type"`
			MsgID     uint64 `json:"msg_id"`
			InReplyTo uint64 `json:"in_reply_to"`
			Code      int    `json:"code"`
			Text      string `json:"text"`
			Echo      string `json:"echo"`
		}
		if err := json.Unmarshal(env.Body, &body); err != nil {
			t.Fatal(err)
		}
		wantRequestID := uint64(10)
		if i == 2 {
			wantRequestID = 41
		}
		if body.Type != wantType || body.MsgID != uint64(i+1) || body.InReplyTo != wantRequestID {
			t.Fatalf("response %d: %#v", i, body)
		}
		if i == 1 && (body.Code != codeAlreadyInitialized || body.Text == "") {
			t.Fatal("missing repeated-init error details")
		}
		if i == 2 && body.Echo != "hello" {
			t.Fatal("echo changed")
		}
	}
	var extra Envelope
	if err := decoder.Decode(&extra); err != io.EOF {
		t.Fatalf("unexpected extra stdout: %v", err)
	}
}

func TestNodeRejectsEchoBeforeInit(t *testing.T) {
	n := NewNode()
	var req Envelope
	if err := json.Unmarshal([]byte(echoInput(t, 41, "hello")), &req); err != nil {
		t.Fatal(err)
	}
	if _, err := n.Handle(req, 1); err == nil || !strings.Contains(err.Error(), "not initialized") {
		t.Fatalf("expected lifecycle error, got %v", err)
	}
	if n.id != "" || len(n.members) != 0 {
		t.Fatal("uninitialized request changed state")
	}
}

func TestNodeInvalidInitLeavesStateAvailableForRetry(t *testing.T) {
	cases := map[string]string{
		"missing identity":   `{"type":"init","msg_id":10,"node_ids":["n1"]}`,
		"wrong destination":  `{"type":"init","msg_id":10,"node_id":"n2","node_ids":["n2"]}`,
		"missing self":       `{"type":"init","msg_id":10,"node_id":"n1","node_ids":["n2"]}`,
		"empty members":      `{"type":"init","msg_id":10,"node_id":"n1","node_ids":[]}`,
		"wrong members type": `{"type":"init","msg_id":10,"node_id":"n1","node_ids":"n1"}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			n := NewNode()
			req := initEnvelope(t, 10)
			req.Body = json.RawMessage(body)
			if _, err := n.Handle(req, 1); err == nil {
				t.Fatal("invalid initialization accepted")
			}
			if n.id != "" || len(n.members) != 0 {
				t.Fatal("failed initialization changed state")
			}
			if _, err := n.Handle(initEnvelope(t, 11), 2); err != nil {
				t.Fatalf("valid retry failed: %v", err)
			}
			if n.id != "n1" || !reflect.DeepEqual(n.members, []string{"n1", "n2", "n3"}) {
				t.Fatal("retry did not store complete identity")
			}
		})
	}
}

func TestNodeRejectsInvalidRequestsWithoutChangingIdentity(t *testing.T) {
	cases := map[string]Envelope{
		"wrong destination":   {Src: "c1", Dest: "n2", Body: json.RawMessage(`{"type":"echo","msg_id":41,"echo":"hello"}`)},
		"missing source":      {Dest: "n1", Body: json.RawMessage(`{"type":"echo"}`)},
		"missing destination": {Src: "c1", Body: json.RawMessage(`{"type":"echo"}`)},
		"malformed header":    {Src: "c1", Dest: "n1", Body: json.RawMessage(`{`)},
		"unknown operation":   {Src: "c1", Dest: "n1", Body: json.RawMessage(`{"type":"unknown"}`)},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			n := NewNode()
			if _, err := n.Handle(initEnvelope(t, 10), 1); err != nil {
				t.Fatal(err)
			}
			if _, err := n.Handle(req, 2); err == nil {
				t.Fatal("invalid request accepted")
			}
			if n.id != "n1" || !reflect.DeepEqual(n.members, []string{"n1", "n2", "n3"}) {
				t.Fatal("rejection changed identity")
			}
		})
	}
}

func TestNodeInstancesAndRunSessionsAreIndependent(t *testing.T) {
	first, second := NewNode(), NewNode()
	if _, err := first.Handle(initEnvelope(t, 10), 1); err != nil {
		t.Fatal(err)
	}
	req := Envelope{Src: "c1", Dest: "n2", Body: json.RawMessage(`{"type":"init","msg_id":10,"node_id":"n2","node_ids":["n2"]}`)}
	if _, err := second.Handle(req, 1); err != nil {
		t.Fatal(err)
	}
	if first.id != "n1" || second.id != "n2" {
		t.Fatal("node identities are shared")
	}
	second.members[0] = "changed"
	if first.members[0] != "n1" {
		t.Fatal("membership storage is shared")
	}
	var a, b bytes.Buffer
	for _, out := range []*bytes.Buffer{&a, &b} {
		if err := Run(strings.NewReader(initLine(t)+echoInput(t, 41, "hello")), out); err != nil {
			t.Fatal(err)
		}
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatal("fresh sessions did not reset state and IDs")
	}
}

type failSecondWrite struct {
	calls int
	err   error
}

func (w *failSecondWrite) Write(p []byte) (int, error) {
	w.calls++
	if w.calls == 1 {
		return len(p), nil
	}
	return 0, w.err
}

func TestRunEchoWriteFailureAfterInitialization(t *testing.T) {
	failure := errors.New("echo write failed")
	writer := &failSecondWrite{err: failure}
	input := initLine(t) + echoInput(t, 41, "first") + "\n" + echoInput(t, 42, "later") + "\n"
	if err := Run(strings.NewReader(input), writer); !errors.Is(err, failure) {
		t.Fatalf("error = %v, want echo write failure", err)
	}
	if writer.calls != 2 {
		t.Fatalf("write calls = %d, want init and one echo attempt", writer.calls)
	}
}

func TestRunReadFailureAfterInitialization(t *testing.T) {
	failure := errors.New("read failed after init")
	var out bytes.Buffer
	input := io.MultiReader(strings.NewReader(initLine(t)), failingReader{failure})
	if err := Run(input, &out); !errors.Is(err, failure) {
		t.Fatalf("error = %v, want read failure", err)
	}
	replies := readReplies(t, out.Bytes())
	if len(replies) != 1 || replies[0].Body.Type != "init_ok" {
		t.Fatal("expected only initialization acknowledgement")
	}
}

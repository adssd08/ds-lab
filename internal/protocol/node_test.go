package protocol

import (
	"encoding/json"
	"reflect"
	"testing"
)

func initEnvelope(t *testing.T, msgID uint64) Envelope {
	t.Helper()

	body, err := json.Marshal(InitRequest{
		Type:    "init",
		MsgID:   msgID,
		NodeID:  "n1",
		NodeIDs: []string{"n1", "n2", "n3"},
	})

	if err != nil {
		t.Fatal(err)
	}

	return Envelope{Src: "c1", Dest: "n1", Body: body}
}

func TestNodeInitializesOnlyOnce(t *testing.T) {
	node := NewNode()

	response, err := node.Handle(initEnvelope(t, 10), 1)
	if err != nil {
		t.Fatal(err)
	}

	var ack InitResponse
	if err := json.Unmarshal(response.Body, &ack); err != nil {
		t.Fatal(err)
	}

	wantAck := InitResponse{
		Type:      "init_ok",
		MsgID:     1,
		InReplyTo: 10,
	}

	if response.Src != "n1" || response.Dest != "c1" || ack != wantAck {
		t.Fatalf("unexpected initialization response: %#v, %#v", response, ack)
	}

	repeated := initEnvelope(t, 11)
	repeated.Body = json.RawMessage(
		`{"type":"init","msg_id":11,"node_id":"n2","node_ids":["n2"]}`,
	)

	response, err = node.Handle(repeated, 2)
	if err != nil {
		t.Fatal(err)
	}

	var rejected ErrorResponse
	if err := json.Unmarshal(response.Body, &rejected); err != nil {
		t.Fatal(err)
	}

	if response.Src != "n1" || response.Dest != "c1" || rejected.Type != "error" || rejected.Code != codeAlreadyInitialized ||
		rejected.MsgID != 2 || rejected.InReplyTo != 11 {
		t.Fatalf("unexpected repeated-init response: %#v, %#v", response, rejected)
	}

	if node.id != "n1" {
		t.Fatalf("identity changed to %q", node.id)
	}

	wantMembers := []string{"n1", "n2", "n3"}

	if !reflect.DeepEqual(node.members, wantMembers) {
		t.Fatalf("membership changed: %v", node.members)
	}

}

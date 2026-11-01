package protocol

import (
	"encoding/json"
	"fmt"
)

const codeAlreadyInitialized = 1000

// Node owns initialization state for one stream and is used by one goroutine.
// An empty id means uninitialized; valid initialization sets it once.
type Node struct {
	id      string
	members []string
}

func NewNode() *Node {
	return &Node{}
}

// Handle returns a reply, including recoverable protocol errors, or a fatal Go error.
func (n *Node) Handle(request Envelope, replyID uint64) (Envelope, error) {
	if request.Src == "" || request.Dest == "" {
		return Envelope{}, fmt.Errorf("source and destination are required")
	}

	var header Header
	if err := json.Unmarshal(request.Body, &header); err != nil {
		return Envelope{}, fmt.Errorf("decode message header: %w", err)
	}

	if header.Type == "init" {
		return n.initialize(request, header, replyID)
	}

	if n.id == "" {
		return Envelope{}, fmt.Errorf("node is not initialized")
	}

	if request.Dest != n.id {
		return Envelope{}, fmt.Errorf("message addressed to %q, node is %q", request.Dest, n.id)
	}

	switch header.Type {
	case "echo":
		return Echo(request, replyID)

	default:
		return Envelope{}, fmt.Errorf("unsupported message type: %q", header.Type)
	}
}

func (n *Node) initialize(request Envelope, header Header, replyID uint64) (Envelope, error) {
	if n.id != "" {
		return makeReply(n.id, request.Src, ErrorResponse{
			Type:      "error",
			MsgID:     replyID,
			InReplyTo: header.MsgID,
			Code:      codeAlreadyInitialized,
			Text:      "node is already initialized",
		})
	}

	var body InitRequest
	if err := json.Unmarshal(request.Body, &body); err != nil {
		return Envelope{}, fmt.Errorf("decode init body: %w", err)
	}

	if body.NodeID == "" {
		return Envelope{}, fmt.Errorf("node_id is required")
	}

	if body.NodeID != request.Dest {
		return Envelope{}, fmt.Errorf("node_id must match the envelope destination")
	}

	foundSelf := false
	for _, member := range body.NodeIDs {
		if member == body.NodeID {
			foundSelf = true
			break
		}
	}

	if !foundSelf {
		return Envelope{}, fmt.Errorf("node_ids must contain this node")
	}

	response, err := makeReply(body.NodeID, request.Src, InitResponse{
		Type:      "init_ok",
		MsgID:     replyID,
		InReplyTo: body.MsgID,
	})

	if err != nil {
		return Envelope{}, err
	}

	// Validate and construct the acknowledgement before committing state.
	n.id = body.NodeID
	n.members = append([]string(nil), body.NodeIDs...)

	return response, nil
}

func makeReply(src, dest string, body any) (Envelope, error) {
	encoded, err := json.Marshal(body)
	if err != nil {
		return Envelope{}, fmt.Errorf("encode response body: %w", err)
	}

	return Envelope{
		Src:  src,
		Dest: dest,
		Body: encoded,
	}, nil
}

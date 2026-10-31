package protocol

import (
	"encoding/json"
	"fmt"
)

// Echo builds a reply without reading or writing process streams.
func Echo(request Envelope, replyID uint64) (Envelope, error) {
	if request.Src == "" || request.Dest == "" {
		return Envelope{}, fmt.Errorf("source and destination are required")
	}

	var body EchoRequest
	if err := json.Unmarshal(request.Body, &body); err != nil {
		return Envelope{}, fmt.Errorf("decode echo body: %w", err)
	}

	if body.Type != "echo" {
		return Envelope{}, fmt.Errorf("unsupported message type: %q", body.Type)
	}

	responseBody, err := json.Marshal(EchoResponse{
		Type:      "echo_ok",
		MsgID:     replyID,
		InReplyTo: body.MsgID,
		Echo:      body.Echo,
	})

	if err != nil {
		return Envelope{}, fmt.Errorf("encode echo body: %w", err)
	}

	return Envelope{
		Src:  request.Dest,
		Dest: request.Src,
		Body: responseBody,
	}, nil
}

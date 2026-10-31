// Package protocol implements the stdin/stdout message protocol.
package protocol

import "encoding/json"

// Envelope contains routing information and an operation-specific JSON body.
type Envelope struct {
	Src  string          `json:"src"`
	Dest string          `json:"dest"`
	Body json.RawMessage `json:"body"`
}

type EchoRequest struct {
	Type  string `json:"type"`
	MsgID uint64 `json:"msg_id"`
	Echo  string `json:"echo"`
}

type EchoResponse struct {
	Type      string `json:"type"`
	MsgID     uint64 `json:"msg_id"`
	InReplyTo uint64 `json:"in_reply_to"`
	Echo      string `json:"echo"`
}

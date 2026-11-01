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

// Header contains fields needed before selecting an operation handler.
type Header struct {
	Type  string `json:"type"`
	MsgID uint64 `json:"msg_id"`
}

type InitRequest struct {
	Type    string   `json:"type"`
	MsgID   uint64   `json:"msg_id"`
	NodeID  string   `json:"node_id"`
	NodeIDs []string `json:"node_ids"`
}

type InitResponse struct {
	Type      string `json:"type"`
	MsgID     uint64 `json:"msg_id"`
	InReplyTo uint64 `json:"in_reply_to"`
}

type ErrorResponse struct {
	Type      string `json:"type"`
	MsgID     uint64 `json:"msg_id"`
	InReplyTo uint64 `json:"in_reply_to"`
	Code      int    `json:"code"`
	Text      string `json:"text"`
}

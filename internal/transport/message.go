// Package transport provides message delivery and controlled fault injection.
// Its current implementations must be used from a single goroutine.
package transport

import "errors"

// Message is an application-level message between named nodes.
type Message struct {
	ID   string
	From string
	To   string
	Body string
}

func (m Message) validate() error {
	if m.ID == "" || m.From == "" || m.To == "" {
		return errors.New("message ID, source, and destination are required")
	}

	return nil
}

// Sender accepts a message for transmission.
// A nil error does not guarantee that the destination received it.
type Sender interface {
	Send(Message) error
}

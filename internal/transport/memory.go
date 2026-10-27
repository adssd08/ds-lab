package transport

// Memory delivers messages immediately into destination inboxes.
type Memory struct {
	inboxes map[string][]Message
}

func NewMemory() *Memory {
	return &Memory{
		inboxes: make(map[string][]Message),
	}
}

func (m *Memory) Send(msg Message) error {
	if err := msg.validate(); err != nil {
		return err
	}

	m.inboxes[msg.To] = append(m.inboxes[msg.To], msg)
	return nil
}

// Inbox returns a copy so callers cannot overwrite stored messages.
func (m *Memory) Inbox(nodeId string) []Message {
	return append([]Message(nil), m.inboxes[nodeId]...)
}

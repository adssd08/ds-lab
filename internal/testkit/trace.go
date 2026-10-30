package testkit

import "time"

// Event describes an observation at a logical simulation timestamp.
type Event struct {
	At     time.Duration
	Kind   string
	Detail string
}

type Trace struct {
	clock  *FakeClock
	events []Event
}

func NewTrace(clock *FakeClock) *Trace {
	return &Trace{clock: clock}
}

func (t *Trace) Add(kind, detail string) {
	t.events = append(t.events, Event{
		At:     t.clock.Now(),
		Kind:   kind,
		Detail: detail,
	})
}

// Events returns a snapshot that callers can inspect without changing the log.
func (t *Trace) Events() []Event {
	return append([]Event(nil), t.events...)
}

package transport

import "fmt"

const maxPending = 1024

type link struct {
	from string
	to   string
}

// Rule controls one directed link.
type Rule struct {
	Drop       bool
	Hold       bool
	Duplicates int
}

// Faults wraps a Sender with deterministic delivery controls.
type Faults struct {
	next    Sender
	rules   map[link]Rule
	pending []Message
}

// NewFaults requires a non-nil underlying sender.
func NewFaults(next Sender) *Faults {
	return &Faults{
		next:  next,
		rules: make(map[link]Rule),
	}
}

func (f *Faults) SetRule(from, to string, rule Rule) error {
	if from == "" || to == "" {
		return fmt.Errorf("rule endpoints are required")
	}

	if rule.Duplicates < 0 || rule.Duplicates > 8 {
		return fmt.Errorf("duplicates must be between 0 and 8")
	}

	f.rules[link{from: from, to: to}] = rule
	return nil
}

func (f *Faults) Send(msg Message) error {
	if err := msg.validate(); err != nil {
		return err
	}

	rule := f.rules[link{from: msg.From, to: msg.To}]

	// Loss is invisible to the sender in this simulation.
	if rule.Drop {
		return nil
	}

	copies := 1 + rule.Duplicates

	if rule.Hold {
		if copies > maxPending-len(f.pending) {
			return fmt.Errorf(("pending message limit reached"))
		}

		for i := 0; i < copies; i++ {
			f.pending = append(f.pending, msg)
		}

		return nil
	}

	for i := 0; i < copies; i++ {
		if err := f.next.Send(msg); err != nil {
			return err
		}
	}

	return nil
}

func (f *Faults) Pending() int {
	return len(f.pending)
}

// Release removes a held message and attempts delivery.
// Choosing the index lets tests control delivery order.
func (f *Faults) Release(index int) error {
	if index < 0 || index >= len(f.pending) {
		return fmt.Errorf("pending index out of range: %d", index)
	}

	msg := f.pending[index]
	rule := f.rules[link{from: msg.From, to: msg.To}]

	if !rule.Drop {
		if err := f.next.Send(msg); err != nil {
			return err
		}
	}

	f.pending = append(f.pending[:index], f.pending[index+1:]...)
	return nil
}

package transport

import (
	"errors"
	"reflect"
	"testing"
)

func requireInbox(t *testing.T, memory *Memory, node string, want []Message) {
	t.Helper()

	if got := memory.Inbox(node); !reflect.DeepEqual(got, want) {
		t.Fatalf("inbox %s = %#v, want %#v", node, got, want)
	}
}

func TestFaultDelivery(t *testing.T) {
	msg := Message{
		ID:   "m1",
		From: "A",
		To:   "B",
		Body: "hello",
	}

	tests := []struct {
		name string
		rule Rule
		want []Message
	}{
		{
			name: "normal",
			want: []Message{msg},
		},
		{
			name: "drop",
			rule: Rule{Drop: true},
			want: nil,
		},
		{
			name: "duplicate",
			rule: Rule{Duplicates: 1},
			want: []Message{msg, msg},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			memory := NewMemory()
			faults := NewFaults(memory)

			if err := faults.SetRule("A", "B", tt.rule); err != nil {
				t.Fatal(err)
			}

			if err := faults.Send(msg); err != nil {
				t.Fatal(err)
			}

			requireInbox(t, memory, "B", tt.want)
		})
	}
}

func TestHeldMessagesCanBeReordered(t *testing.T) {
	memory := NewMemory()
	faults := NewFaults(memory)

	if err := faults.SetRule("A", "B", Rule{Hold: true}); err != nil {
		t.Fatal(err)
	}

	first := Message{ID: "m1", From: "A", To: "B", Body: "first"}
	second := Message{ID: "m2", From: "A", To: "B", Body: "second"}

	for _, msg := range []Message{first, second} {
		if err := faults.Send(msg); err != nil {
			t.Fatal(err)
		}
	}

	requireInbox(t, memory, "B", nil)

	if got := faults.Pending(); got != 2 {
		t.Fatalf("pending = %d, want 2", got)
	}

	if err := faults.Release(1); err != nil {
		t.Fatal(err)
	}

	if err := faults.Release(0); err != nil {
		t.Fatal(err)
	}

	requireInbox(t, memory, "B", []Message{second, first})

	if got := faults.Pending(); got != 0 {
		t.Fatalf("pending = %d , want 0", got)
	}

}

func TestPartitionPreservesOtherLinks(t *testing.T) {
	memory := NewMemory()
	faults := NewFaults(memory)

	for _, endpoints := range [][2]string{{"A", "B"}, {"B", "A"}} {
		if err := faults.SetRule(endpoints[0], endpoints[1], Rule{Drop: true}); err != nil {
			t.Fatal(err)
		}
	}

	ab := Message{ID: "ab", From: "A", To: "B", Body: "blocked"}
	ba := Message{ID: "ba", From: "B", To: "A", Body: "blocked"}
	bc := Message{ID: "bc", From: "B", To: "C", Body: "allowed"}
	cb := Message{ID: "cb", From: "C", To: "B", Body: "allowed"}

	for _, msg := range []Message{ab, ba, bc, cb} {
		if err := faults.Send(msg); err != nil {
			t.Fatal(err)
		}
	}

	requireInbox(t, memory, "A", nil)
	requireInbox(t, memory, "B", []Message{cb})
	requireInbox(t, memory, "C", []Message{bc})

	if err := faults.SetRule("A", "B", Rule{}); err != nil {
		t.Fatal(err)
	}

	healed := Message{
		ID: "after-heal", From: "A", To: "B", Body: "allowed again",
	}

	if err := faults.Send(healed); err != nil {
		t.Fatal(err)
	}

	requireInbox(t, memory, "B", []Message{cb, healed})
}

func TestMessageIDMissing(t *testing.T) {
	msg := Message{From: "A", To: "B", Body: "ID Missing"}

	if err := msg.validate(); err == nil {
		t.Fatalf("Message validated without ID")
	}
}

func TestDuplicateCount(t *testing.T) {
	memory := NewMemory()
	faults := NewFaults(memory)

	if err := faults.SetRule("A", "B", Rule{Duplicates: 9}); err == nil {
		t.Fatalf("Rule acepted with more than max duplicates")
	}

	if err := faults.SetRule("A", "B", Rule{Duplicates: -1}); err == nil {
		t.Fatalf("Rule accepted with negative duplicates")
	}
}

// Invalid messages must fail at both transport entry points without side effects.
func TestInvalidMessagesAreNotDelivered(t *testing.T) {
	for _, field := range []string{"ID", "From", "To"} {
		t.Run(field, func(t *testing.T) {
			msg := Message{ID: "m1", From: "A", To: "B"}
			switch field {
			case "ID":
				msg.ID = ""
			case "From":
				msg.From = ""
			case "To":
				msg.To = ""
			}

			memory := NewMemory()
			faults := NewFaults(memory)
			if err := faults.SetRule("A", "B", Rule{Hold: true}); err != nil {
				t.Fatal(err)
			}
			for _, sender := range []Sender{memory, faults} {
				if err := sender.Send(msg); err == nil {
					t.Fatal("invalid message accepted")
				}
			}
			requireInbox(t, memory, "B", nil)
			requireInbox(t, memory, "", nil)
			if faults.Pending() != 0 {
				t.Fatal("invalid message was queued")
			}
		})
	}
}

func TestInvalidRulesDoNotReplaceExistingRule(t *testing.T) {
	tests := []struct {
		name, from, to string
		rule           Rule
	}{
		{"missing source", "", "B", Rule{}},
		{"missing destination", "A", "", Rule{}},
		{"negative duplicates", "A", "B", Rule{Duplicates: -1}},
		{"excessive duplicates", "A", "B", Rule{Duplicates: 9}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			memory := NewMemory()
			faults := NewFaults(memory)
			if err := faults.SetRule("A", "B", Rule{Drop: true}); err != nil {
				t.Fatal(err)
			}
			if err := faults.SetRule(tt.from, tt.to, tt.rule); err == nil {
				t.Fatal("invalid rule accepted")
			}
			if err := faults.Send(Message{ID: "m1", From: "A", To: "B"}); err != nil {
				t.Fatal(err)
			}
			requireInbox(t, memory, "B", nil)
		})
	}
}

func TestInvalidReleaseDoesNotChangeQueue(t *testing.T) {
	memory := NewMemory()
	faults := NewFaults(memory)
	if err := faults.Release(0); err == nil {
		t.Fatal("release from empty queue succeeded")
	}
	if err := faults.SetRule("A", "B", Rule{Hold: true}); err != nil {
		t.Fatal(err)
	}
	msg := Message{ID: "m1", From: "A", To: "B"}
	if err := faults.Send(msg); err != nil {
		t.Fatal(err)
	}
	for _, index := range []int{-1, 1, 100} {
		if err := faults.Release(index); err == nil {
			t.Fatalf("release(%d) succeeded", index)
		}
		if faults.Pending() != 1 {
			t.Fatal("invalid release changed queue size")
		}
		requireInbox(t, memory, "B", nil)
	}
	if err := faults.Release(0); err != nil {
		t.Fatal(err)
	}
	requireInbox(t, memory, "B", []Message{msg})
}

func TestPartitionDropsPreviouslyHeldMessage(t *testing.T) {
	memory := NewMemory()
	faults := NewFaults(memory)
	if err := faults.SetRule("A", "B", Rule{Hold: true}); err != nil {
		t.Fatal(err)
	}
	if err := faults.Send(Message{ID: "m1", From: "A", To: "B"}); err != nil {
		t.Fatal(err)
	}
	if err := faults.SetRule("A", "B", Rule{Drop: true}); err != nil {
		t.Fatal(err)
	}
	if err := faults.Release(0); err != nil {
		t.Fatal(err)
	}
	if faults.Pending() != 0 {
		t.Fatal("dropped message remained pending")
	}
	requireInbox(t, memory, "B", nil)
	// Healing permits new messages but does not resurrect the discarded message.
	if err := faults.SetRule("A", "B", Rule{}); err != nil {
		t.Fatal(err)
	}
	if err := faults.Release(0); err == nil {
		t.Fatal("discarded message could be released again")
	}
}

func TestDropTakesPrecedenceOverHoldAndDuplicates(t *testing.T) {
	memory := NewMemory()
	faults := NewFaults(memory)
	if err := faults.SetRule("A", "B", Rule{Drop: true, Hold: true, Duplicates: 8}); err != nil {
		t.Fatal(err)
	}
	if err := faults.Send(Message{ID: "m1", From: "A", To: "B"}); err != nil {
		t.Fatal(err)
	}
	if faults.Pending() != 0 {
		t.Fatal("dropped message was held")
	}
	requireInbox(t, memory, "B", nil)
}

func TestHeldDuplicatesAreDeliveredOnlyOncePerCopy(t *testing.T) {
	memory := NewMemory()
	faults := NewFaults(memory)
	if err := faults.SetRule("A", "B", Rule{Hold: true, Duplicates: 8}); err != nil {
		t.Fatal(err)
	}
	msg := Message{ID: "m1", From: "A", To: "B", Body: "same identity"}
	if err := faults.Send(msg); err != nil {
		t.Fatal(err)
	}
	if got := faults.Pending(); got != 9 {
		t.Fatalf("pending = %d, want 9", got)
	}
	requireInbox(t, memory, "B", nil)
	var want []Message
	for i := 0; i < 9; i++ {
		if err := faults.Release(0); err != nil {
			t.Fatal(err)
		}
		want = append(want, msg)
	}
	if faults.Pending() != 0 {
		t.Fatal("released messages remained pending")
	}
	requireInbox(t, memory, "B", want)
}

// Overflow must reject an entire copy batch without discarding existing messages.
func TestPendingCapacityIsAtomicAndReusable(t *testing.T) {
	memory := NewMemory()
	faults := NewFaults(memory)
	if err := faults.SetRule("A", "B", Rule{Hold: true}); err != nil {
		t.Fatal(err)
	}
	fill := Message{ID: "fill", From: "A", To: "B"}
	for i := 0; i < maxPending-1; i++ {
		if err := faults.Send(fill); err != nil {
			t.Fatal(err)
		}
	}
	if err := faults.SetRule("A", "B", Rule{Hold: true, Duplicates: 1}); err != nil {
		t.Fatal(err)
	}
	rejected := Message{ID: "rejected", From: "A", To: "B"}
	if err := faults.Send(rejected); err == nil {
		t.Fatal("copy batch exceeding capacity accepted")
	}
	if got := faults.Pending(); got != maxPending-1 {
		t.Fatalf("partial batch changed queue: pending = %d", got)
	}
	if err := faults.SetRule("A", "B", Rule{Hold: true}); err != nil {
		t.Fatal(err)
	}
	last := Message{ID: "last", From: "A", To: "B"}
	if err := faults.Send(last); err != nil {
		t.Fatal(err)
	}
	if err := faults.Send(rejected); err == nil {
		t.Fatal("full queue accepted another message")
	}
	if faults.Pending() != maxPending {
		t.Fatal("full queue size changed")
	}
	requireInbox(t, memory, "B", nil)
	if err := faults.Release(0); err != nil {
		t.Fatal(err)
	}
	reused := Message{ID: "reused", From: "A", To: "B"}
	if err := faults.Send(reused); err != nil {
		t.Fatalf("released capacity was not reusable: %v", err)
	}
	for faults.Pending() > 0 {
		if err := faults.Release(0); err != nil {
			t.Fatal(err)
		}
	}
	want := make([]Message, maxPending-1)
	for i := range want {
		want[i] = fill
	}
	want = append(want, last, reused)
	requireInbox(t, memory, "B", want)
}

func TestHealingOneDirectionLeavesReverseBlocked(t *testing.T) {
	memory := NewMemory()
	faults := NewFaults(memory)
	for _, pair := range [][2]string{{"A", "B"}, {"B", "A"}} {
		if err := faults.SetRule(pair[0], pair[1], Rule{Drop: true}); err != nil {
			t.Fatal(err)
		}
	}
	if err := faults.SetRule("A", "B", Rule{}); err != nil {
		t.Fatal(err)
	}
	forward := Message{ID: "forward", From: "A", To: "B"}
	reverse := Message{ID: "reverse", From: "B", To: "A"}
	for _, msg := range []Message{forward, reverse} {
		if err := faults.Send(msg); err != nil {
			t.Fatal(err)
		}
	}
	requireInbox(t, memory, "B", []Message{forward})
	requireInbox(t, memory, "A", nil)
}

// senderFunc adapts a function into a Sender for controlled downstream failures.
type senderFunc func(Message) error

func (s senderFunc) Send(msg Message) error { return s(msg) }

func TestSendPropagatesErrorAfterPartialDelivery(t *testing.T) {
	failure := errors.New("downstream unavailable")
	memory := NewMemory()
	attempts := 0
	faults := NewFaults(senderFunc(func(msg Message) error {
		attempts++
		if attempts == 2 {
			return failure
		}
		return memory.Send(msg)
	}))
	if err := faults.SetRule("A", "B", Rule{Duplicates: 2}); err != nil {
		t.Fatal(err)
	}
	msg := Message{ID: "m1", From: "A", To: "B"}
	if err := faults.Send(msg); !errors.Is(err, failure) {
		t.Fatalf("Send error = %v, want downstream failure", err)
	}
	if attempts != 2 {
		t.Fatalf("send attempts = %d, want 2", attempts)
	}
	requireInbox(t, memory, "B", []Message{msg})
	if faults.Pending() != 0 {
		t.Fatal("direct send unexpectedly queued messages")
	}
}

func TestReleaseFailurePreservesMessageForRetry(t *testing.T) {
	failure := errors.New("downstream unavailable")
	memory := NewMemory()
	unavailable := true
	faults := NewFaults(senderFunc(func(msg Message) error {
		if unavailable {
			return failure
		}
		return memory.Send(msg)
	}))
	if err := faults.SetRule("A", "B", Rule{Hold: true}); err != nil {
		t.Fatal(err)
	}
	msg := Message{ID: "m1", From: "A", To: "B"}
	if err := faults.Send(msg); err != nil {
		t.Fatal(err)
	}
	if err := faults.Release(0); !errors.Is(err, failure) {
		t.Fatalf("Release error = %v, want downstream failure", err)
	}
	if faults.Pending() != 1 {
		t.Fatal("failed release removed the message")
	}
	requireInbox(t, memory, "B", nil)
	// This fake fails before delivery. A real error might follow remote acceptance.
	unavailable = false
	if err := faults.Release(0); err != nil {
		t.Fatal(err)
	}
	if faults.Pending() != 0 {
		t.Fatal("successful retry left the message pending")
	}
	requireInbox(t, memory, "B", []Message{msg})
}

func TestInboxSnapshotCannotOverwriteStoredMessages(t *testing.T) {
	memory := NewMemory()
	msg := Message{ID: "m1", From: "A", To: "B", Body: "original"}
	if err := memory.Send(msg); err != nil {
		t.Fatal(err)
	}
	snapshot := memory.Inbox("B")
	snapshot[0].Body = "modified by caller"
	requireInbox(t, memory, "B", []Message{msg})
}

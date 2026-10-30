package testkit

import (
	"ds-lab/internal/transport"
	"fmt"
	"reflect"
	"testing"
	"time"
)

// tracedDelivery records successful delivery into the destination inbox.
type tracedDelivery struct {
	memory *transport.Memory
	trace  *Trace
}

func (d *tracedDelivery) Send(msg transport.Message) error {
	if err := d.memory.Send(msg); err != nil {
		return err
	}

	d.trace.Add("delivered", msg.ID)
	return nil
}

func runScenario(t *testing.T, seed int64) ([]Event, []transport.Message) {
	t.Helper()

	clock := NewFakeClock()
	rng := NewRNG(seed)
	trace := NewTrace(clock)
	memory := transport.NewMemory()

	delivery := &tracedDelivery{
		memory: memory,
		trace:  trace,
	}

	faults := transport.NewFaults(delivery)

	must(t, faults.SetRule("A", "B", transport.Rule{Hold: true}))

	for _, id := range []string{"m1", "m2"} {
		msg := transport.Message{ID: id, From: "A", To: "B"}
		must(t, faults.Send(msg))
		trace.Add("queued", id)
	}

	release := func() {
		trace.Add("release", fmt.Sprintf("pending=%d", faults.Pending()))
		must(t, faults.Release(0))
	}

	for i := 0; i < 2; i++ {
		delay := time.Duration(5+rng.Intn(4)) * time.Millisecond
		must(t, clock.AfterFunc(delay, release))
	}

	must(t, clock.AfterFunc(2*time.Millisecond, func() {
		must(t, faults.SetRule("A", "B", transport.Rule{Drop: true}))
		trace.Add("partition", "A->B")
	}))

	must(t, clock.AfterFunc(10*time.Millisecond, func() {
		must(t, faults.SetRule("A", "B", transport.Rule{Hold: true}))
		trace.Add("heal", "A->B")

		msg := transport.Message{ID: "m3", From: "A", To: "B"}
		must(t, faults.Send(msg))
		trace.Add("queued", msg.ID)
	}))

	must(t, clock.AfterFunc(12*time.Millisecond, release))
	must(t, clock.Advance(20*time.Millisecond))

	if got := faults.Pending(); got != 0 {
		t.Fatalf("pending = %d, want 0", got)
	}

	return trace.Events(), memory.Inbox("B")
}

func TestFaultScenarioIsReproducible(t *testing.T) {
	const seed int64 = 42

	firstTrace, firstInbox := runScenario(t, seed)
	secondTrace, secondInbox := runScenario(t, seed)

	if !reflect.DeepEqual(firstTrace, secondTrace) {
		t.Fatalf("seed %d produced different traces:\n%#v,\n%#v", seed, firstTrace, secondTrace)
	}

	wantInbox := []transport.Message{
		{ID: "m3", From: "A", To: "B"},
	}

	for _, inbox := range [][]transport.Message{firstInbox, secondInbox} {
		if !reflect.DeepEqual(inbox, wantInbox) {
			t.Fatalf("inbox = %#v, want %#v", inbox, wantInbox)
		}
	}

	deliveries := 0

	for _, event := range firstTrace {
		t.Logf("%s %-10s %s", event.At, event.Kind, event.Detail)

		if event.Kind == "delivered" {
			deliveries++
			if event.At != 12*time.Millisecond || event.Detail != "m3" {
				t.Fatalf("unexpected delivery: %#v", event)
			}
		}

	}

	if deliveries != 1 {
		t.Fatalf("deliveries = %d, want 1", deliveries)
	}
}

// A repeated trace could be consistently wrong. This fixture checks the expected
// transition sequence as well as logical timestamps for the documented seed.
func TestFaultScenarioMatchesExpectedTrace(t *testing.T) {
	got, _ := runScenario(t, 42)
	want := []Event{
		{At: 0, Kind: "queued", Detail: "m1"},
		{At: 0, Kind: "queued", Detail: "m2"},
		{At: 2 * time.Millisecond, Kind: "partition", Detail: "A->B"},
		{At: 6 * time.Millisecond, Kind: "release", Detail: "pending=2"},
		{At: 8 * time.Millisecond, Kind: "release", Detail: "pending=1"},
		{At: 10 * time.Millisecond, Kind: "heal", Detail: "A->B"},
		{At: 10 * time.Millisecond, Kind: "queued", Detail: "m3"},
		{At: 12 * time.Millisecond, Kind: "release", Detail: "pending=1"},
		{At: 12 * time.Millisecond, Kind: "delivered", Detail: "m3"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("trace = %#v, want %#v", got, want)
	}
}

func TestFaultScenarioReplaysOtherSeeds(t *testing.T) {
	for _, seed := range []int64{0, -7} {
		t.Run(fmt.Sprintf("seed=%d", seed), func(t *testing.T) {
			first, inbox := runScenario(t, seed)
			second, secondInbox := runScenario(t, seed)
			if !reflect.DeepEqual(first, second) {
				t.Fatalf("seed %d did not reproduce its trace", seed)
			}
			want := []transport.Message{{ID: "m3", From: "A", To: "B"}}
			if !reflect.DeepEqual(inbox, want) || !reflect.DeepEqual(secondInbox, want) {
				t.Fatalf("seed %d produced incorrect final inboxes: %#v, %#v", seed, inbox, secondInbox)
			}
		})
	}
}

func TestTracedDeliveryDoesNotLogRejectedMessage(t *testing.T) {
	clock := NewFakeClock()
	trace := NewTrace(clock)
	memory := transport.NewMemory()
	delivery := &tracedDelivery{memory: memory, trace: trace}
	if err := delivery.Send(transport.Message{From: "A", To: "B"}); err == nil {
		t.Fatal("message without an ID was accepted")
	}
	if len(trace.Events()) != 0 || len(memory.Inbox("B")) != 0 {
		t.Fatal("rejected message produced a delivery observation")
	}
}

package testkit

import (
	"reflect"
	"testing"
	"time"
)

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestClockPreservesEqualDeadlineOrder(t *testing.T) {
	clock := NewFakeClock()
	var order []string

	must(t, clock.AfterFunc(5*time.Millisecond, func() {
		order = append(order, "first")

		must(t, clock.AfterFunc(0, func() {
			order = append(order, "nested")
		}))
	}))

	must(t, clock.AfterFunc(5*time.Millisecond, func() {
		order = append(order, "second")
	}))

	must(t, clock.Advance(4*time.Millisecond))
	if len(order) != 0 {
		t.Fatalf("callback ran early %v", order)
	}

	must(t, clock.Advance(time.Millisecond))

	want := []string{"first", "second", "nested"}

	if !reflect.DeepEqual(order, want) {
		t.Fatalf("order = %v, want %v", order, want)
	}

	if got := clock.Now(); got != 5*time.Millisecond {
		t.Fatalf("time = %v, want 5ms", got)
	}

	must(t, clock.Advance(5*time.Millisecond))
	if got := clock.Now(); got != 10*time.Millisecond {
		t.Fatalf("time = %v, want 10ms", got)
	}

}

// Rejected scheduling must leave the clock and previously queued work intact.
func TestClockRejectsInvalidScheduling(t *testing.T) {
	for _, name := range []string{"negative delay", "nil callback"} {
		t.Run(name, func(t *testing.T) {
			clock := NewFakeClock()
			calls := 0
			must(t, clock.AfterFunc(5*time.Millisecond, func() { calls++ }))
			var err error
			if name == "negative delay" {
				err = clock.AfterFunc(-time.Millisecond, func() { calls += 100 })
			} else {
				err = clock.AfterFunc(time.Millisecond, nil)
			}
			if err == nil {
				t.Fatal("invalid schedule accepted")
			}
			if got := clock.Now(); got != 0 {
				t.Fatalf("invalid scheduling changed time to %v", got)
			}
			must(t, clock.Advance(5*time.Millisecond))
			if calls != 1 {
				t.Fatalf("callbacks = %d, want only the original callback", calls)
			}
		})
	}
}

func TestClockRejectsBackwardAdvanceWithoutLosingWork(t *testing.T) {
	clock := NewFakeClock()
	must(t, clock.Advance(3*time.Millisecond))
	calls := 0
	must(t, clock.AfterFunc(2*time.Millisecond, func() { calls++ }))
	if err := clock.Advance(-time.Millisecond); err == nil {
		t.Fatal("negative advance accepted")
	}
	if got := clock.Now(); got != 3*time.Millisecond {
		t.Fatalf("time changed after rejected advance: %v", got)
	}
	must(t, clock.Advance(2*time.Millisecond))
	if calls != 1 {
		t.Fatalf("callbacks = %d, want 1", calls)
	}
}

func TestClockAdvancesWithoutScheduledWork(t *testing.T) {
	clock := NewFakeClock()
	must(t, clock.Advance(7*time.Millisecond))
	if got := clock.Now(); got != 7*time.Millisecond {
		t.Fatalf("time = %v, want 7ms", got)
	}
	must(t, clock.Advance(0))
	if got := clock.Now(); got != 7*time.Millisecond {
		t.Fatalf("zero advance changed time: %v", got)
	}
}

func TestClockZeroAdvanceRunsDueWorkExactlyOnce(t *testing.T) {
	clock := NewFakeClock()
	must(t, clock.Advance(3*time.Millisecond))
	calls := 0
	must(t, clock.AfterFunc(0, func() {
		calls++
		if got := clock.Now(); got != 3*time.Millisecond {
			t.Fatalf("callback observed time %v, want 3ms", got)
		}
	}))
	if calls != 0 {
		t.Fatal("AfterFunc ran the callback before Advance")
	}
	must(t, clock.Advance(0))
	must(t, clock.Advance(0))
	if calls != 1 {
		t.Fatalf("callbacks = %d, want 1", calls)
	}
}

// Insert out of order and advance partway: future work must remain queued.
func TestClockRunsAtDeadlinesAndPreservesFutureWork(t *testing.T) {
	clock := NewFakeClock()
	var observed []time.Duration
	for _, delay := range []time.Duration{9, 3, 6} {
		must(t, clock.AfterFunc(delay*time.Millisecond, func() {
			observed = append(observed, clock.Now())
		}))
	}
	must(t, clock.Advance(8*time.Millisecond))
	want := []time.Duration{3 * time.Millisecond, 6 * time.Millisecond}
	if !reflect.DeepEqual(observed, want) {
		t.Fatalf("callback times = %v, want %v", observed, want)
	}
	if got := clock.Now(); got != 8*time.Millisecond {
		t.Fatalf("time = %v, want advance target 8ms", got)
	}
	must(t, clock.Advance(time.Millisecond))
	want = append(want, 9*time.Millisecond)
	if !reflect.DeepEqual(observed, want) {
		t.Fatalf("callback times = %v, want %v", observed, want)
	}
	must(t, clock.Advance(time.Millisecond))
	if len(observed) != 3 {
		t.Fatal("completed callback ran again")
	}
}

func TestClockNestedSchedulingUsesCallbackDeadline(t *testing.T) {
	clock := NewFakeClock()
	var observed []time.Duration
	must(t, clock.AfterFunc(3*time.Millisecond, func() {
		observed = append(observed, clock.Now())
		must(t, clock.AfterFunc(2*time.Millisecond, func() {
			observed = append(observed, clock.Now())
		}))
	}))
	must(t, clock.Advance(10*time.Millisecond))
	want := []time.Duration{3 * time.Millisecond, 5 * time.Millisecond}
	if !reflect.DeepEqual(observed, want) {
		t.Fatalf("callback times = %v, want %v", observed, want)
	}
	if got := clock.Now(); got != 10*time.Millisecond {
		t.Fatalf("time = %v, want 10ms", got)
	}
}

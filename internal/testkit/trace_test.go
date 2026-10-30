package testkit

import (
	"reflect"
	"testing"
	"time"
)

func TestTraceRecordsLogicalTimeAndInsertionOrder(t *testing.T) {
	clock := NewFakeClock()
	trace := NewTrace(clock)
	if got := len(trace.Events()); got != 0 {
		t.Fatalf("new trace length = %d, want 0", got)
	}
	trace.Add("start", "A")
	must(t, clock.Advance(5*time.Millisecond))
	trace.Add("queued", "m1")
	trace.Add("delivered", "m1")
	want := []Event{
		{At: 0, Kind: "start", Detail: "A"},
		{At: 5 * time.Millisecond, Kind: "queued", Detail: "m1"},
		{At: 5 * time.Millisecond, Kind: "delivered", Detail: "m1"},
	}
	if got := trace.Events(); !reflect.DeepEqual(got, want) {
		t.Fatalf("events = %#v, want %#v", got, want)
	}
}

func TestTraceSnapshotsCannotChangeStoredEvents(t *testing.T) {
	clock := NewFakeClock()
	trace := NewTrace(clock)
	trace.Add("queued", "m1")
	snapshot := trace.Events()
	snapshot[0] = Event{At: time.Hour, Kind: "modified", Detail: "bad"}
	snapshot = append(snapshot, Event{Kind: "extra"})
	want := []Event{{At: 0, Kind: "queued", Detail: "m1"}}
	if got := trace.Events(); !reflect.DeepEqual(got, want) {
		t.Fatalf("snapshot mutation changed trace: %#v", got)
	}
	trace.Add("delivered", "m1")
	if snapshot[0].Kind != "modified" || len(snapshot) != 2 {
		t.Fatal("later recording changed the returned snapshot")
	}
}

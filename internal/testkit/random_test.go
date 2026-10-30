package testkit

import (
	"fmt"
	"testing"
)

func TestRNGRepeatsSequenceForSameSeed(t *testing.T) {
	for _, seed := range []int64{0, 42, -7} {
		t.Run(fmt.Sprintf("seed=%d", seed), func(t *testing.T) {
			first, second := NewRNG(seed), NewRNG(seed)
			for i := 0; i < 10; i++ {
				got, want := first.Int63(), second.Int63()
				if got != want {
					t.Fatalf("draw %d = %d, want %d", i, got, want)
				}
			}
		})
	}
}

func TestRNGInstancesDoNotShareState(t *testing.T) {
	const seed int64 = 42
	busy, untouched, reference := NewRNG(seed), NewRNG(seed), NewRNG(seed)
	for i := 0; i < 20; i++ {
		busy.Int63()
	}
	for i := 0; i < 10; i++ {
		if got, want := untouched.Int63(), reference.Int63(); got != want {
			t.Fatalf("other generator changed draw %d: got %d, want %d", i, got, want)
		}
	}
	// Reconstructing the RNG resets it to the start of the sequence.
	reset, fresh := NewRNG(seed), NewRNG(seed)
	if got, want := reset.Int63(), fresh.Int63(); got != want {
		t.Fatalf("reset draw = %d, want %d", got, want)
	}
}

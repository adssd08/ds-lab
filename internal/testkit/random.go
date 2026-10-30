package testkit

import "math/rand"

// NewRNG returns an independent, reproducible generator for a scenario.
// It is not cryptographically secure and must be used by one goroutine.
func NewRNG(seed int64) *rand.Rand {
	return rand.New(rand.NewSource(seed))
}

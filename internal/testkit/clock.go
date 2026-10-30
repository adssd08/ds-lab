// Package testkit provides deterministic, single-goroutine test helpers.
package testkit

import (
	"fmt"
	"sort"
	"time"
)

type scheduled struct {
	at  time.Duration
	run func()
}

// FakeClock measures logical time elapsed since the simulation began.
// Callbacks run synchronously during Advance, never in background goroutines.
type FakeClock struct {
	now   time.Duration
	tasks []scheduled
}

func NewFakeClock() *FakeClock {
	return &FakeClock{}
}

func (c *FakeClock) Now() time.Duration {
	return c.now
}

// AfterFunc queues a callback relative to the current logical time.
func (c *FakeClock) AfterFunc(delay time.Duration, fn func()) error {
	if delay < 0 {
		return fmt.Errorf("delay must not be negative")
	}

	if fn == nil {
		return fmt.Errorf("callback is required")
	}

	c.tasks = append(c.tasks, scheduled{
		at:  c.now + delay,
		run: fn,
	})

	return nil
}

// Advance executes due callbacks in deadline order.
// Equal deadlines preserve insertion order.
// Callbacks may schedule work, but must not call Advance recursively.
func (c *FakeClock) Advance(delta time.Duration) error {
	if delta < 0 {
		return fmt.Errorf("advance must not be negative")
	}

	target := c.now + delta

	for {
		sort.SliceStable(c.tasks, func(i, j int) bool {
			return c.tasks[i].at < c.tasks[j].at
		})

		if len(c.tasks) == 0 || c.tasks[0].at > target {
			break
		}

		task := c.tasks[0]
		c.tasks = c.tasks[1:]

		c.now = task.at
		task.run()

	}

	c.now = target
	return nil

}

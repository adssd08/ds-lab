// Command launcher starts independent node processes and owns their cleanup.
// It targets macOS and similar Unix systems through SIGINT and SIGTERM handling.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

// main converts run errors into a failing process status after deferred cleanup.
func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

// run starts the configured nodes, then waits for cancellation or a child exit.
// Any child exit during normal operation fails the run and stops its siblings.
func run() error {
	nodeBin := flag.String("node-bin", "./bin/node", "node executable")
	count := flag.Int("n", 3, "number of nodes")
	basePort := flag.Int("base-port", 9001, "first node port")
	flag.Parse()

	if *count < 1 || *basePort < 1 || *basePort > 65535 {
		return fmt.Errorf("invalid node count or base port")
	}

	// Check the full port range without first adding possibly oversized values.
	if *count > 65536-*basePort {
		return fmt.Errorf("node ports exceed 65535")
	}

	// Pass an explicit filesystem path to exec.Command rather than a PATH lookup.
	binary, err := filepath.Abs(*nodeBin)

	if err != nil {
		return fmt.Errorf("resolve node executable: %w", err)
	}

	// A signal to the launcher requests coordinated shutdown of all children.
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)

	defer stop()

	// Each run gets an isolated root; each child gets its own directory below it.
	// These directories are temporary, so this is not a durable restart setup.
	root, err := os.MkdirTemp("", "ds-lab-")

	if err != nil {
		return fmt.Errorf("create temporary root: %w", err)
	}

	// Defers run in reverse order: shutdown registered below finishes first.
	defer os.RemoveAll(root)

	// Only this goroutine owns commands and remaining. Child waiters report
	// through exits, so the bookkeeping needs no shared-memory mutex.
	commands := make([]*exec.Cmd, 0, *count)
	// Each child sends once. Buffering all results avoids blocking waiters
	// while the launcher is still starting other nodes.
	exits := make(chan error, *count)
	remaining := 0

	// The closure reads final bookkeeping values at return time. A direct deferred
	// call here would capture the initial empty slice and zero remaining count.
	// This cleanup also handles failure partway through startup.
	defer func() {
		shutdown(commands, exits, remaining)
	}()

	for i := 0; i < *count; i++ {
		// Avoid starting additional nodes after a shutdown request is observed.
		if ctx.Err() != nil {
			return nil
		}

		id := fmt.Sprintf("n%d", i+1)
		dir := filepath.Join(root, id)

		if err := os.Mkdir(dir, 0700); err != nil {
			return fmt.Errorf("create directory for %s: %w", id, err)
		}

		addr := "127.0.0.1:" + strconv.Itoa(*basePort+i)
		// Execute the built node directly, without a shell or go run wrapper.
		cmd := exec.Command(
			binary,
			"-id", id,
			"-addr", addr,
			"-data-dir", dir,
		)

		// Inherit output streams so startup and shutdown diagnostics stay visible.
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		// Start returns after process creation; it does not prove the port is ready.
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("start %s: %w", id, err)
		}

		commands = append(commands, cmd)
		remaining++

		// Pair every successful Start with exactly one Wait to reap the child.
		// One waiter per process lets us observe exits in any order.
		go func(id string, cmd *exec.Cmd) {
			err := cmd.Wait()
			exits <- fmt.Errorf("node %s exited unexpectedly: %v", id, err)
		}(id, cmd)
	}

	log.Printf("started %d processes; data root=%s", len(commands), root)

	select {
	case <-ctx.Done():
		return nil
	case err := <-exits:
		// This event is already consumed, so shutdown must wait for one fewer result.
		remaining--
		return err
	}
}

// shutdown signals all children before waiting, giving them a shared grace period.
// remaining counts exit events not yet consumed, not necessarily live processes.
// After the deadline it forces termination and still waits for every Wait result.
// Forced termination is logged, but does not change run's return value.
func shutdown(commands []*exec.Cmd, exits <-chan error, remaining int) {
	// Already-exited children may be present; their exit events still need draining.
	for _, cmd := range commands {
		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			if !os.IsNotExist(err) && err != os.ErrProcessDone {
				log.Printf("singal pid=%d: %v", cmd.Process.Pid, err)
			}
		}
	}

	// One timer bounds the cooperative phase for the group, not five seconds per child.
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()

	for remaining > 0 {
		select {
		case <-exits:
			remaining--
		case <-timer.C:
			log.Print("shutdown deadline exceeded; killing remaining processes")

			for _, cmd := range commands {
				_ = cmd.Process.Kill()
			}

			// Kill requests termination; draining results confirms Wait has completed.
			for remaining > 0 {
				<-exits
				remaining--
			}

		}
	}
}

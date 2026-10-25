// Command node runs one independently addressable TCP node until signaled to stop.
package main

import (
	"context"
	"ds-lab/internal/node"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
)

// main reports failures after run has returned and executed its deferred cleanup.
// os.Exit itself does not execute deferred functions.
func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

// run parses configuration and translates OS signals into node cancellation.
func run() error {
	id := flag.String("id", "", "node ID")
	addr := flag.String("addr", "127.0.0.1:9001", "TCP listen address")
	dataDir := flag.String("data-dir", "", "existing data directory")
	flag.Parse()

	// Ctrl+C sends SIGINT; the launcher requests shutdown with SIGTERM.
	// Both cancel ctx. Cancellation requests shutdown; node.Run waits for it to finish.
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)

	// Release signal notifications when the node returns.
	defer stop()

	return node.Run(ctx, *id, *addr, *dataDir)
}

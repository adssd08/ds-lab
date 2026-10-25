// Package node implements the TCP listener lifecycle for a single lab node.
package node

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
)

// Run serves TCP connection probes until ctx is canceled or Accept fails.
// The caller owns dataDir: it must already exist and is not removed here.
// Connections are closed immediately; no application protocol or storage is implemented yet.
func Run(ctx context.Context, id, addr, dataDir string) error {
	if id == "" || dataDir == "" {
		return errors.New("node ID and data directory are required")
	}

	// Validate the launcher-provided directory before acquiring the network resource.
	info, err := os.Stat(dataDir)
	if err != nil {
		return fmt.Errorf("inspect data directory: %w", err)
	}

	if !info.IsDir() {
		return errors.New("data path must be a directory")
	}

	// Binding is the authoritative port-availability check. A prior check could race
	// with another process acquiring the port before this call.
	listener, err := net.Listen("tcp", addr)

	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}

	// Cover every return path, including unexpected accept errors.
	defer listener.Close()

	log.Printf(
		"node=%s pid=%d listening=%s data=%s",
		id, os.Getpid(), listener.Addr(), dataDir,
	)

	// The accept goroutine sends exactly one result. One buffer slot lets it report
	// an error even before the main goroutine is ready to receive.
	done := make(chan error, 1)

	// Accept blocks, so use a goroutine while the main goroutine watches cancellation.
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				done <- err
				return
			}
			// A successful connection is enough for today's port-readiness probe.
			conn.Close()
		}
	}()

	select {
	case err := <-done:
		return fmt.Errorf("accept conection: %w", err)
	case <-ctx.Done():
		// Context cancellation alone does not unblock Accept. Closing the listener does.
		listener.Close()

		// Join the accept goroutine before returning. net.ErrClosed is expected
		// because this shutdown branch deliberately closed the listener.
		err := <-done
		if err != nil && !errors.Is(err, net.ErrClosed) {
			return fmt.Errorf("close listener: %w", err)
		}

		log.Printf("node=%s stopped", id)
		return nil
	}
}

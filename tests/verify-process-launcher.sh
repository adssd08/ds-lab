#!/bin/bash

# Build and check that three local nodes accept TCP connections, then release
# their ports after the launcher receives SIGTERM. Requires Bash, make, Go, and nc.
# Run with: bash tests/verify-process-launcher.sh
# Use otherwise unused ports: TCP connectivity alone does not establish node identity.

# Exit on unhandled command failures and unset variables. Commands used as if
# conditions are exempt from errexit, so their failure branches must be correct.
set -eu

# Resolve paths relative to the repository even when invoked from elsewhere.
cd "$(dirname "$0")/.."

launcher_pid=""
count=3
base_port=19001

# Also stop and reap the launcher if readiness checks fail before normal shutdown.
cleanup() {
    if [ -n "$launcher_pid" ]; then
        kill -TERM "$launcher_pid" 2>/dev/null || true
        wait "$launcher_pid" 2>/dev/null || true
    fi
}

trap cleanup EXIT

make build

./bin/launcher -n $count -base-port $base_port &
# $! identifies the background launcher, which owns its node processes.
launcher_pid=$!

# Poll because successful process creation does not guarantee a bound port yet.
# Each connection attempt has its own timeout; this is not a strict total deadline.
for port in $(seq $base_port $((base_port + count - 1))); do
    ready=0

    for attempt in {1..50}; do
        if nc -z -w 1 127.0.0.1 "$port"; then
            ready=1
            break
        fi
        sleep 0.1
    done

    if [ "$ready" -ne 1 ]; then
        echo "FAIL: port $port did not become ready"
        exit 1
    fi
done

echo "All three ports accept connections"

# Signal only the launcher, exercising its responsibility to stop all children.
# wait checks successful launcher exit; logs distinguish graceful versus forced stops.
kill -TERM "$launcher_pid"
wait "$launcher_pid"
# Prevent the EXIT trap from signaling an already-reaped process.
launcher_pid=""

# Refused connections verify port closure; this script does not inspect directories.
for port in $(seq $base_port $((base_port + count - 1))); do
    if nc -z -w 1 127.0.0.1 "$port"; then
        echo "FAIL: port $port still accepts connections"
        exit 1
    fi
done

echo "PASS: launcher exited and all three ports are closed"
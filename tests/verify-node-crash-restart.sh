#!/bin/bash
set -eu

cd "$(dirname "$0")/.."

node_pid=""
data_dir=$(mktemp -d)
port=19101

cleanup(){
    if [ -n "$node_pid" ]; then
        kill -TERM "$node_pid" 2>/dev/null || true
        wait "$node_pid" 2>/dev/null || true
    fi
    rm -rf "$data_dir"
}

trap cleanup EXIT

make build

if nc -z -w 1 127.0.0.1 "$port"; then
    echo "FAIL: port $port is already occupied"
    exit 1
fi

start_node(){
    ./bin/node \
        -id restart-test \
        -addr "127.0.0.1:$port" \
        -data-dir "$data_dir" &
    node_pid=$!

    for attempt in {1..50}; do
        if ! kill -0 "$node_pid" 2>/dev/null; then
            echo "FAIL: node exited during startup"
            return 1
        fi

        if nc -z -w 1 127.0.0.1 "$port"; then
            return 0
        fi

        sleep 0.1
    done

    echo "FAIL: node did not become ready"
    return 1
}

printf 'preserve me\n' > "$data_dir/marker"
start_node

kill -KILL "$node_pid"

if wait "$node_pid"; then
    echo "FAIL: killed node exited successfully"
    exit 1
fi
node_pid=""

if nc -z -w 1 127.0.0.1 "$port"; then
    echo "FAIL: port remains open after crash"
    exit 1
fi

start_node

if [ "$(cat "$data_dir/marker")" != "preserve me" ]; then
    echo "FAIL: data-directory marker changed"
    exit 1
fi

kill -TERM "$node_pid"
wait "$node_pid"
node_pid=""


if nc -z -w 1 127.0.0.1 "$port"; then
    echo "FAIL: port remains open after shutdown"
    exit 1
fi

echo "PASS: node crashed, restarted with its directory, and stopped cleanly"
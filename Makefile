# Run these targets even if a file with the same name exists.
.PHONY: build test race

# Build each executable explicitly; generated binaries stay under ignored bin/.
build:
	mkdir -p bin
	go build -o bin/ds-lab ./cmd/ds-lab
	go build -o bin/node ./cmd/node
	go build -o bin/launcher ./cmd/launcher
	go build -o bin/stdio-node ./cmd/stdio-node

# Discover Go tests recursively. Shell integration checks are run separately.
test:
	go test ./...

# Instrument executed Go tests; packages without tests get no lifecycle coverage.
# Process-level checks need separately race-instrumented binaries to cover children.
race:
	go test -race ./...
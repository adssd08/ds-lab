.PHONY: build test race

build:
	mkdir -p bin
	go build -o bin/ds-lab ./cmd/ds-lab

test:
	go test ./...

race:
	go test -race ./...
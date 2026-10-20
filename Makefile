PREFIX ?= $(HOME)/.local

.PHONY: build test install

build:
	go build -o bin/lazydev ./cmd/lazydev

test:
	go vet ./...
	go test ./...

install:
	go build -o $(PREFIX)/bin/lazydev ./cmd/lazydev

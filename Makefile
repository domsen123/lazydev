PREFIX ?= $(HOME)/.local
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X lazydev/internal/version.Version=$(VERSION)

.PHONY: build test install

build:
	go build -ldflags "$(LDFLAGS)" -o bin/lazydev ./cmd/lazydev

test:
	go vet ./...
	go test ./...

install:
	go build -ldflags "$(LDFLAGS)" -o $(PREFIX)/bin/lazydev ./cmd/lazydev

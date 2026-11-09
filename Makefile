PREFIX ?= $(HOME)/.local
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X lazydev/internal/version.Version=$(VERSION)

.PHONY: build test test-mod install

build:
	go build -ldflags "$(LDFLAGS)" -o bin/lazydev ./cmd/lazydev

test:
	go vet ./...
	go test ./...

# test-mod validates the Claude Code mod and runs its tests in a temporary copy. It needs claude.
test-mod:
	@tmp=$$(mktemp -d) && cp -r internal/mod/files/. $$tmp/ && cp internal/mod/testdata/*.test.tsx $$tmp/hooks/ && \
		claude plugin validate $$tmp && claude plugin test $$tmp; status=$$?; rm -rf $$tmp; exit $$status

install:
	go build -ldflags "$(LDFLAGS)" -o $(PREFIX)/bin/lazydev ./cmd/lazydev

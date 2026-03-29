GO ?= go

.PHONY: build install test install-daemon

build:
	$(GO) build -o bin/lumn ./cmd/lumn

install:
	$(GO) install ./cmd/lumn

install-daemon:
	$(GO) install ./cmd/lumnd

test:
	$(GO) test ./...

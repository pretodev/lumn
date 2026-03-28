GO ?= go

.PHONY: build install test install-daemon

build:
	$(GO) build -o bin/lumn .

install:
	$(GO) install .

install-daemon:
	$(GO) install ./cmd/lumnd

test:
	$(GO) test ./...

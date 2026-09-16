BINARY  := dist/genesis
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
ARGS    ?=

.PHONY: build run dev test test-go test-web fmt vet tidy clean

build:
	@mkdir -p dist
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/genesis
	@echo "built $(BINARY) ($(VERSION))"

run: build
	./$(BINARY) $(ARGS)

dev:
	go run ./cmd/genesis $(ARGS)

test: test-go test-web

test-go:
	go test ./...

test-web:
	cd web && node test/render.test.mjs

fmt:
	gofmt -w cmd internal

vet:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -rf dist tmpdata tmpcheck

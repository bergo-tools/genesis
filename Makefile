BINARY  := dist/genesis
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
ARGS    ?=

.PHONY: build run dev test fmt vet tidy clean

build:
	@mkdir -p dist
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/genesis
	@echo "built $(BINARY) ($(VERSION))"

run: build
	./$(BINARY) $(ARGS)

dev:
	go run ./cmd/genesis $(ARGS)

test:
	go test ./...

fmt:
	gofmt -w cmd internal

vet:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -rf dist tmpdata tmpcheck

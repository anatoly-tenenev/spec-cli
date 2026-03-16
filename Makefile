BINARY := spec-cli
CMD := ./cmd/spec-cli
VERSION ?= dev
LDFLAGS ?= -X github.com/anatoly-tenenev/spec-cli/internal/buildinfo.Version=$(VERSION)

.PHONY: fmt vet test build run

fmt:
	go fmt ./...

vet:
	go vet ./...

test:
	go test ./...

build:
	mkdir -p bin
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) $(CMD)

run:
	go run $(CMD) $(ARGS)

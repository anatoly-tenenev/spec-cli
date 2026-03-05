BINARY := spec-cli
CMD := ./cmd/spec-cli

.PHONY: fmt vet test build run

fmt:
	go fmt ./...

vet:
	go vet ./...

test:
	go test ./...

build:
	mkdir -p bin
	go build -o bin/$(BINARY) $(CMD)

run:
	go run $(CMD) $(ARGS)

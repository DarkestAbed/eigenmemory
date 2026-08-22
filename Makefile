.PHONY: build test lint clean install

BINARY_NAME := eigenmemory
MAIN_PACKAGE := ./cmd/eigenmemory

build:
	go build -o $(BINARY_NAME) $(MAIN_PACKAGE)

test:
	go test -v ./...

lint:
	golangci-lint run ./...

clean:
	rm -f $(BINARY_NAME)

install:
	go install $(MAIN_PACKAGE)

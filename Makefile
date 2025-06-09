# Makefile for FHIR Validation Proxy

BINARY=fhir-validation-proxy
CMD=./cmd/server

.PHONY: all build run test lint lint-fix clean

all: build

build:
	go build -o $(BINARY) $(CMD)

run: build
	./$(BINARY)

test:
	go test ./...

lint:
	golangci-lint run ./...

lint-fix:
	golangci-lint run --fix ./...

clean:
	rm -f $(BINARY) 
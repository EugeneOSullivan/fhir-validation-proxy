# Makefile for FHIR Validation Proxy

BINARY=fhir-validation-proxy
CMD=./cmd/server

.PHONY: all build run test lint clean

all: build

build:
	go build -o $(BINARY) $(CMD)

run: build
	./$(BINARY)

test:
	go test ./...

lint:
	golangci-lint run || echo 'Install golangci-lint for linting.'

clean:
	rm -f $(BINARY) 
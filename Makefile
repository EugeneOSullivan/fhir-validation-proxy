# Makefile for FHIR Validation Proxy

BINARY=fhir-validation-proxy
CMD=./cmd/server

.PHONY: all build run test lint lint-fix clean coverage

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

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated at coverage.html"
	@echo "Open coverage.html in your browser to view the report" 
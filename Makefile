# Makefile for the storm STORM research CLI
# All Go commands use GOTOOLCHAIN=local to pin at go1.24.4.

BINARY     := storm
CMD        := ./cmd/storm
GOTOOLCHAIN := GOTOOLCHAIN=local

.PHONY: all build test vet fmt lint docker clean help

all: build

## build: Compile the storm binary
build:
	$(GOTOOLCHAIN) go build -o $(BINARY) $(CMD)

## test: Run all tests (no live network — uses httptest mocks)
test:
	$(GOTOOLCHAIN) go test ./... -count=1

## vet: Run go vet across all packages
vet:
	$(GOTOOLCHAIN) go vet ./...

## fmt: Format all Go source files in-place
fmt:
	gofmt -w .

## lint: Check formatting + vet (gate: must both pass for CI)
lint: vet
	@if [ -n "$$(gofmt -l .)" ]; then \
		echo "gofmt: the following files are not formatted:"; \
		gofmt -l .; \
		exit 1; \
	fi
	@echo "lint: OK"

## docker: Build the Docker image (requires Docker daemon)
docker:
	docker build -t storm:latest . \
	  || echo "docker daemon absent or build failed — see RUNBOOK.md"

## clean: Remove built binary and generated output
clean:
	rm -f $(BINARY)
	rm -rf out/

## help: Show this help
help:
	@grep -E '^## [a-z]' Makefile | sed 's/## /  make /'

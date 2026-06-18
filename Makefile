.PHONY: dev build build-all test lint clean

GO ?= go
GOPATH ?= $(shell $(GO) env GOPATH)
GOLANGCI_LINT ?= $(GOPATH)/bin/golangci-lint

MODULE := github.com/KleaSCM/nala
BUILD_DIR := build

dev:
	wails dev

build:
	wails build -o $(BUILD_DIR)/nala

build-all:
	$(GO) build -o $(BUILD_DIR)/nalad ./cmd/nalad/

test:
	$(GO) test ./... -cover -count=1

lint:
	$(GOLANGCI_LINT) run ./...

clean:
	rm -rf $(BUILD_DIR)
	rm -f nala*.tar.gz

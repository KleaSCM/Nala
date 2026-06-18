.PHONY: dev build build-all test lint clean

GO ?= go
GOPATH ?= $(shell $(GO) env GOPATH)
GOLANGCI_LINT ?= $(GOPATH)/bin/golangci-lint

MODULE := github.com/KleaSCM/nala
BUILD_DIR := build

TAGS ?= webkit2_41
GDK_BACKEND ?= x11

dev:
	GDK_BACKEND=$(GDK_BACKEND) wails dev -tags $(TAGS)

build:
	GDK_BACKEND=$(GDK_BACKEND) wails build -o $(BUILD_DIR)/nala -tags $(TAGS)

build-all:
	$(GO) build -o $(BUILD_DIR)/nalad ./cmd/nalad/

test:
	$(GO) test ./... -cover -count=1

lint:
	$(GOLANGCI_LINT) run ./...

clean:
	rm -rf $(BUILD_DIR)
	rm -f nala*.tar.gz

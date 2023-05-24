#!/usr/bin/env bash

##@ General

.PHONY: help
help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Build

GO_TAGS = -tags=''

GO_SRCS := $(shell find . -type f -name '*.go' -not -name '*_test.go' -not -name 'zz_generated*')

.PHONY: all
all: clean lint test build ## Run all commands to build the tool

.PHONY: clean
clean: ## Clean the bin directory
	rm -rf bin

.PHONY: test
test: ## Run the unit tests
	go test $(GO_TAGS) -race -v ./...

.PHONY: build
build: static bin/helmbin ## Build helmbin binaries

GO_ASMFLAGS = -asmflags "all=-trimpath=$(shell dirname $(PWD))"
GO_GCFLAGS = -gcflags "all=-trimpath=$(shell dirname $(PWD))"
LD_FLAGS = -ldflags " \
	-X main.goos=$(shell go env GOOS) \
	-X main.goarch=$(shell go env GOARCH) \
	-X main.gitCommit=$(shell git rev-parse HEAD) \
	-X main.buildDate=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ') \
	"
bin/helmbin: $(GO_SRCS) go.sum
	mkdir -p bin
	go build $(GO_GCFLAGS) $(GO_ASMFLAGS) $(LD_FLAGS) $(GO_TAGS) -o bin/helmbin ./cmd/helmbin

##@ Development

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter
	$(GOLANGCI_LINT) run

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	$(GOLANGCI_LINT) run --fix

GOLANGCI_LINT = $(shell pwd)/bin/golangci-lint
golangci-lint:
	@[ -f $(GOLANGCI_LINT) ] || { \
	set -e ;\
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell dirname $(GOLANGCI_LINT));\
	}

static: static/bin/k0s static/bin/k3s ## Build static assets

static/bin/k0s:
	@mkdir -p static/bin
	@curl -sSL -o static/bin/k0s https://github.com/k0sproject/k0s/releases/download/v1.27.2%2Bk0s.0/k0s-v1.27.2+k0s.0-amd64
	chmod +x static/bin/k0s

static/bin/k3s:
	@mkdir -p static/bin
	@curl -sSL -o static/bin/k3s https://github.com/k3s-io/k3s/releases/download/v1.27.1%2Bk3s1/k3s
	chmod +x static/bin/k3s

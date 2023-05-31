#!/usr/bin/env bash

include embedded-bins/Makefile.variables
include inttest/Makefile.variables

BIN_DIR := $(shell pwd)/bin
export PATH := $(BIN_DIR):$(PATH)

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
	rm -rf $(BIN_DIR)
	rm -rf static/bin/k0s
	rm -rf static/helm/*tgz
	$(MAKE) -C inttest clean

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
BIN = bin/helmbin
bin/helmbin: $(GO_SRCS) go.sum
	@mkdir -p bin
	CGO_ENABLED=0 go build $(GO_GCFLAGS) $(GO_ASMFLAGS) $(LD_FLAGS) $(GO_TAGS) -o $(BIN) ./cmd/helmbin

static: static/bin/k0s static/helm/000-admin-console-$(admin_console_version).tgz ## Build static assets

static/bin/k0s:
	@mkdir -p static/bin
	@curl -fsSL -o static/bin/k0s https://github.com/k0sproject/k0s/releases/download/$(k0s_version)/k0s-$(k0s_version)-amd64
	chmod +x static/bin/k0s

static/helm/000-admin-console-$(admin_console_version).tgz: helm
	@mkdir -p static/helm
	@helm pull oci://registry.replicated.com/library/admin-console --version=$(admin_console_version)
	mv admin-console-$(admin_console_version).tgz static/helm/000-admin-console-$(admin_console_version).tgz

##@ Development

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter
	golangci-lint run

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	golangci-lint run --fix

.PHONY: test
test: ## Run the unit tests
	go test $(GO_TAGS) -race -v ./...

.PHONY: $(smoketests)
$(smoketests): build
	$(MAKE) -C inttest $@

.PHONY: smoketests
smoketests: $(smoketests)

GOLANGCI_LINT = $(BIN_DIR)/golangci-lint
.PHONY: golangci-lint
golangci-lint:
	@[ -f $(GOLANGCI_LINT) ] || { \
	set -e ;\
	curl -fsSL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell dirname $(GOLANGCI_LINT));\
	}

.PHONY: helm
helm:
	@mkdir -p $(BIN_DIR)
	curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | DESIRED_VERSION=v$(helm_version) HELM_INSTALL_DIR=$(BIN_DIR) bash

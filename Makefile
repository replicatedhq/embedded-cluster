#!/usr/bin/env bash

include hack/tools/Makefile.variables
include embedded-bins/Makefile.variables
include inttest/Makefile.variables

ARCH ?= $(shell go env GOARCH)

BIN_DIR := $(shell pwd)/bin
export PATH := $(BIN_DIR):$(PATH)

##@ General

.PHONY: help
help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Build

# runs on linux even if it's built on mac or windows
TARGET_OS ?= linux
BUILD_GO_FLAGS := -tags osusergo
BUILD_GO_FLAGS += -asmflags "all=-trimpath=$(shell dirname $(PWD))"
BUILD_GO_FLAGS += -gcflags "all=-trimpath=$(shell dirname $(PWD))"
BUILD_GO_LDFLAGS_EXTRA := -extldflags=-static
ifeq ($(shell go env GOOS),darwin)
BUILD_GO_LDFLAGS_EXTRA =
endif

LD_FLAGS := -X main.goos=$(shell go env GOOS)
LD_FLAGS += -X main.goarch=$(shell go env GOARCH)
LD_FLAGS += -X main.gitCommit=$(shell git rev-parse HEAD)
LD_FLAGS += -X main.buildDate=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
LD_FLAGS += $(BUILD_GO_LDFLAGS_EXTRA)

GO_SRCS := $(shell find . -type f -name '*.go' -not -name '*_test.go' -not -name 'zz_generated*')

.PHONY: all
all: clean lint test build ## Run all commands to build the tool

.PHONY: clean
clean: ## Clean the bin directory
	rm -rf $(BIN_DIR)
	rm -rf static/bin
	rm -rf static/helm/*tgz
	$(MAKE) -C inttest clean

.PHONY: build
build: static bin/helmbin ## Build helmbin binaries

bin/helmbin: BIN = bin/helmbin
bin/helmbin: TARGET_OS = linux
bin/helmbin: BUILD_GO_CGO_ENABLED = 0
bin/helmbin: $(GO_SRCS) go.sum
	@mkdir -p bin
	CGO_ENABLED=$(BUILD_GO_CGO_ENABLED) GOOS=$(TARGET_OS) go build $(BUILD_GO_FLAGS) -ldflags='$(LD_FLAGS)' -o $(BIN) ./cmd/helmbin

static: static/bin/k0s static/helm/000-admin-console-$(admin_console_version).tgz ## Build static assets

static/bin/k0s:
	@mkdir -p static/bin
	curl -fsSL -o static/bin/k0s https://github.com/k0sproject/k0s/releases/download/$(k0s_version)/k0s-$(k0s_version)-$(ARCH)
	chmod +x static/bin/k0s

static/helm/000-admin-console-$(admin_console_version).tgz: helm
	@mkdir -p static/helm
	$(HELM) pull oci://registry.replicated.com/library/admin-console --version=$(admin_console_version)
	mv admin-console-$(admin_console_version).tgz static/helm/000-admin-console-$(admin_console_version).tgz

HELM = $(BIN_DIR)/helm
.PHONY: helm
helm:
	@mkdir -p $(BIN_DIR)
	curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | \
		DESIRED_VERSION=v$(helm_version) HELM_INSTALL_DIR=$(BIN_DIR) USE_SUDO=false bash

##@ Development

.PHONY: lint
lint: golangci-lint go.sum ## Run golangci-lint linter
	$(GOLANGCI_LINT) run --timeout=5m

.PHONY: lint-fix
lint-fix: golangci-lint go.sum ## Run golangci-lint linter and perform fixes
	$(GOLANGCI_LINT) run --timeout=5m --fix

.PHONY: test
test: GO_TEST_RACE ?= -race
test: go.sum ## Run the unit tests
	go test $(GO_TEST_RACE) -ldflags='$(LD_FLAGS)' -v ./pkg/...

.PHONY: $(smoketests)
$(smoketests): build
	$(MAKE) -C inttest $@

.PHONY: smoketests
smoketests: $(smoketests)

GOLANGCI_LINT = $(BIN_DIR)/golangci-lint
.PHONY: golangci-lint
golangci-lint:
	@mkdir -p $(BIN_DIR)
	curl -fsSL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | \
		sh -s -- -b $(shell dirname $(GOLANGCI_LINT)) v$(golangci-lint_version)

go.sum: go.mod
	go mod tidy && touch -c -- '$@'

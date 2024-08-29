SHELL := /bin/bash

include ../common.mk

# VERSION defines the project version for the bundle.
# Update this value when you upgrade the version of your project.
# To re-generate a bundle for another specific version without changing the standard setup, you can:
# - use the VERSION as arg of the bundle target (e.g make bundle VERSION=0.0.2)
# - use environment variables to overwrite this value (e.g export VERSION=0.0.2)
VERSION ?= 0.0.1

# Image URL to use all building/pushing image targets
IMG ?= controller:latest
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.26.0

# Set the current user to the GITHUB_USER env variable if it is set, which is useful when using Github Codespaces.
# Otherwise, set it to the current user.
CURRENT_USER := $(if $(GITHUB_USER),$(GITHUB_USER),$(shell id -u -n))

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

export PATH := $(shell pwd)/bin:$(PATH)

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: kustomize controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1" output:crd:artifacts:config=config/crd/bases
	$(KUSTOMIZE) build config/crd > charts/embedded-cluster-operator/charts/crds/templates/resources.yaml

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: manifests fmt vet envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test ./... -coverprofile cover.out

##@ Build

.PHONY: build
build: ## Build manager binary.
	go build \
		-tags osusergo,netgo \
		-ldflags="-s -w -extldflags=-static" \
		-o bin/manager main.go

.PHONY: run
run: manifests fmt vet ## Run a controller from your host.
	go run ./main.go

##@ Build Dependencies

## Tool Binaries
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest

## Tool Versions
KUSTOMIZE_VERSION ?= v3.8.7
CONTROLLER_TOOLS_VERSION ?= v0.14.0

KUSTOMIZE_INSTALL_SCRIPT ?= "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"
.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary. If wrong version is installed, it will be removed before downloading.
$(KUSTOMIZE): $(LOCALBIN)
	@if test -x $(LOCALBIN)/kustomize && ! $(LOCALBIN)/kustomize version | grep -q $(KUSTOMIZE_VERSION); then \
		echo "$(LOCALBIN)/kustomize version is not expected $(KUSTOMIZE_VERSION). Removing it before installing."; \
		rm -rf $(LOCALBIN)/kustomize; \
	fi
	test -s $(LOCALBIN)/kustomize || { curl -Ss $(KUSTOMIZE_INSTALL_SCRIPT) | bash -s -- $(subst v,,$(KUSTOMIZE_VERSION)) $(LOCALBIN); }

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary. If wrong version is installed, it will be overwritten.
$(CONTROLLER_GEN): $(LOCALBIN)
	@test -s $(LOCALBIN)/controller-gen && $(LOCALBIN)/controller-gen --version | grep -q $(CONTROLLER_TOOLS_VERSION) || \
	echo "Installing controller-gen version $(CONTROLLER_TOOLS_VERSION) to $(LOCALBIN)" && \
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION) && \
	test -s $(GOBIN)/controller-gen && \
		ln -sf $(GOBIN)/controller-gen $(LOCALBIN)/controller-gen || \
		ln -sf $(GOBIN)/$(shell go env GOOS)_$(shell go env GOARCH)/controller-gen $(LOCALBIN)/controller-gen

.PHONY: schemas
schemas: fmt controller-gen
	go build ${LDFLAGS} -o bin/schemagen ./schemagen
	./bin/schemagen --output-dir ./schemas

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	@test -s $(LOCALBIN)/setup-envtest || \
	echo "Installing setup-envtest to $(LOCALBIN)" && \
	go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest && \
	test -s $(GOBIN)/setup-envtest && \
		ln -sf $(GOBIN)/setup-envtest $(LOCALBIN)/setup-envtest || \
		ln -sf $(GOBIN)/$(shell go env GOOS)_$(shell go env GOARCH)/setup-envtest $(LOCALBIN)/setup-envtest

GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

export PACKAGE_VERSION ?= $(shell git describe --tags --dirty --match='[0-9]*.[0-9]*.[0-9]*')
export IMAGE_NAMESPACE ?= replicated
export IMAGE_NAME ?= $(IMAGE_NAMESPACE)/embedded-cluster-operator-image
export IMAGE_TAG ?= $(shell echo "$(PACKAGE_VERSION)" | sed 's/+/-/')
export IMAGE ?= $(IMAGE_NAME):$(IMAGE_TAG)
export CHART_REMOTE ?= oci://registry.replicated.com/library
export CHART_VERSION ?= $(shell echo "$(PACKAGE_VERSION)" | sed 's/^v//')

export MELANGE_CONFIG := deploy/melange.tmpl.yaml

.PHONY: melange-build-operator
melange-build-operator:
	$(MAKE) melange-build MELANGE_SOURCE_DIR=../

export APKO_CONFIG := deploy/apko.tmpl.yaml

.PHONY: build-operator-image
build-operator-image: melange-build-operator apko-build

.PHONY: build-and-push-operator-image
build-and-push-operator-image: ARCHS ?= arm64
build-and-push-operator-image: melange-build-operator apko-build-and-publish

.PHONY: build-chart
build-chart:
	cd charts/embedded-cluster-operator && ../../scripts/publish-helm-chart.sh
	mkdir -p build
	echo "$(CHART_REMOTE)/embedded-cluster-operator:$(CHART_VERSION)" > build/chart

.PHONY: build-ttl.sh
build-ttl.sh:
	$(MAKE) build-and-push-operator-image \
		IMAGE=ttl.sh/$(IMAGE)

.PHONY: build-chart-ttl.sh
build-chart-ttl.sh:
	$(MAKE) build-chart \
		IMAGE_NAME=ttl.sh/$(IMAGE_NAME) \
		CHART_REMOTE=oci://ttl.sh/$(IMAGE_NAMESPACE)

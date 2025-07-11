SHELL := /bin/bash

include ../common.mk

## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
.PHONY: generate
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

## Tool Binaries
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen

## Tool Versions
CONTROLLER_TOOLS_VERSION ?= v0.18.0

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary. If wrong version is installed, it will be overwritten.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/controller-gen && $(LOCALBIN)/controller-gen --version | grep -q $(CONTROLLER_TOOLS_VERSION) || \
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: unit-tests
unit-tests:
	go test -v ./...

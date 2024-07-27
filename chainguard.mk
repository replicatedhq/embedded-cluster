SHELL := /bin/bash

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
MELANGE ?= $(LOCALBIN)/melange
APKO ?= $(LOCALBIN)/apko

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

melange: $(MELANGE)
$(MELANGE): $(LOCALBIN)
	go install chainguard.dev/melange@latest && \
		test -s $(GOBIN)/melange && \
		ln -sf $(GOBIN)/melange $(LOCALBIN)/melange

apko: $(APKO)
$(APKO): $(LOCALBIN)
	go install chainguard.dev/apko@latest && \
		test -s $(GOBIN)/apko && \
		ln -sf $(GOBIN)/apko $(LOCALBIN)/apko

CHAINGUARD_TOOLS_USE_DOCKER = 0
ifeq ($(CHAINGUARD_TOOLS_USE_DOCKER),"1")
MELANGE_CACHE_DIR ?= /go/pkg/mod
APKO_CMD = docker run -v $(shell pwd):/work -w /work -v $(shell pwd)/build/.docker:/root/.docker cgr.dev/chainguard/apko
MELANGE_CMD = docker run --privileged --rm -v $(shell pwd):/work -w /work -v "$(shell go env GOMODCACHE)":${MELANGE_CACHE_DIR} cgr.dev/chainguard/melange
else
MELANGE_CACHE_DIR ?= build/.melange-cache
APKO_CMD = apko
MELANGE_CMD = melange
endif

$(MELANGE_CACHE_DIR):
	mkdir -p $(MELANGE_CACHE_DIR)

.PHONY: apko-build
apko-build: ARCHS ?= amd64
apko-build: check-env-IMAGE apko-template
	cd build && ${APKO_CMD} \
		build apko.yaml ${IMAGE} apko.tar \
		--arch ${ARCHS}

.PHONY: apko-build-and-publish
apko-build-and-publish: ARCHS ?= amd64
apko-build-and-publish: check-env-IMAGE apko-template
	cd build && ${APKO_CMD} \
		publish apko.yaml ${IMAGE} \
		--arch ${ARCHS} | tee digest

.PHONY: apko-login
apko-login:
	rm -f build/.docker/config.json
	@ { [ "${PASSWORD}" = "" ] || [ "${USERNAME}" = "" ] ; } || \
	${APKO_CMD} \
		login -u "${USERNAME}" \
		--password "${PASSWORD}" "${REGISTRY}"

.PHONY: melange-build
melange-build: ARCHS ?= amd64
melange-build: MELANGE_SOURCE_DIR ?= .
melange-build: $(MELANGE_CACHE_DIR) melange-template
	${MELANGE_CMD} \
		keygen build/melange.rsa
	${MELANGE_CMD} \
		build build/melange.yaml \
		--arch ${ARCHS} \
		--signing-key build/melange.rsa \
		--cache-dir=$(MELANGE_CACHE_DIR) \
		--source-dir $(MELANGE_SOURCE_DIR) \
		--out-dir build/packages/

.PHONY: melange-template
melange-template: check-env-MELANGE_CONFIG check-env-PACKAGE_VERSION
	mkdir -p build
	envsubst '$${PACKAGE_VERSION}' < ${MELANGE_CONFIG} > build/melange.yaml

.PHONY: apko-template
apko-template: check-env-APKO_CONFIG check-env-PACKAGE_VERSION
	mkdir -p build
	envsubst '$${PACKAGE_NAME} $${PACKAGE_VERSION} $${UPSTREAM_VERSION}' < ${APKO_CONFIG} > build/apko.yaml

check-env-%:
	@ if [ "${${*}}" = "" ]; then \
		echo "Environment variable $* not set"; \
		exit 1; \
	fi

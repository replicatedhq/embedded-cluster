SHELL := /bin/bash

export PATH := $(shell pwd)/bin:$(PATH)

include ../common.mk
include ../versions.mk

OS ?= linux
ARCH ?= $(shell go env GOARCH)

USE_CHAINGUARD ?= 1

export PACKAGE_VERSION ?= $(VERSION)
export IMAGE_NAME ?= replicated/embedded-cluster-local-artifact-mirror

.PHONY: build
build:
	mkdir -p bin
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build \
		-tags osusergo,netgo \
		-ldflags="-s -w -X github.com/replicatedhq/embedded-cluster/pkg/versions.K0sVersion=$(K0S_VERSION) -extldflags=-static" \
		-o bin/local-artifact-mirror-$(OS)-$(ARCH) ../cmd/local-artifact-mirror

.PHONY: build-deps
build-deps:
	$(MAKE) -C .. go.mod crds

export APKO_CONFIG := deploy/apko.tmpl.yaml

.PHONY: melange-build-local-artifact-mirror
melange-build-local-artifact-mirror:
	$(MAKE) melange-build \
		MELANGE_CONFIG=deploy/melange.tmpl.yaml \
		MELANGE_SOURCE_DIR=..

.PHONY: build-local-artifact-mirror-image
build-local-artifact-mirror-image:
	@if [ "$(USE_CHAINGUARD)" == "1" ]; then \
		$(MAKE) build-local-artifact-mirror-image-chainguard; \
	else \
		$(MAKE) build-local-artifact-mirror-image-dockerfile; \
	fi

.PHONY: build-local-artifact-mirror-image-chainguard
build-local-artifact-mirror-image-chainguard:
	$(MAKE) melange-build-local-artifact-mirror
	$(MAKE) apko-build IMAGE="$(IMAGE_NAME):$(call image-tag,$(PACKAGE_VERSION))"

.PHONY: build-local-artifact-mirror-image-dockerfile
build-local-artifact-mirror-image-dockerfile:
	docker build --platform="$(OS)/$(ARCH)" -t "$(IMAGE_NAME):$(call image-tag,$(PACKAGE_VERSION))" -f ../dev/dockerfiles/local-artifact-mirror/Dockerfile.ttlsh ../
	mkdir -p build
	echo "$(IMAGE_NAME):$(call image-tag,$(PACKAGE_VERSION))" > build/image

.PHONY: build-and-push-local-artifact-mirror-image
build-and-push-local-artifact-mirror-image:
	@if [ "$(USE_CHAINGUARD)" == "1" ]; then \
		$(MAKE) build-and-push-local-artifact-mirror-image-chainguard; \
	else \
		$(MAKE) build-and-push-local-artifact-mirror-image-dockerfile; \
	fi

.PHONY: build-and-push-local-artifact-mirror-image-chainguard
build-and-push-local-artifact-mirror-image-chainguard:
	mkdir -p build
ifdef REGISTRY_SERVER
	$(MAKE) apko-login REGISTRY=$(REGISTRY_SERVER) USERNAME=$(REGISTRY_USERNAME) PASSWORD="$(REGISTRY_PASSWORD)"
endif
	$(MAKE) melange-build-local-artifact-mirror
	$(MAKE) apko-build-and-publish IMAGE="$(IMAGE_NAME):$(call image-tag,$(PACKAGE_VERSION))"

.PHONY: build-and-push-local-artifact-mirror-image-dockerfile
build-and-push-local-artifact-mirror-image-dockerfile: build-local-artifact-mirror-image-dockerfile
	docker push "$(IMAGE_NAME):$(call image-tag,$(PACKAGE_VERSION))"

.PHONY: build-ttl.sh
build-ttl.sh: IMAGE_NAME = ttl.sh/$(CURRENT_USER)/embedded-cluster-local-artifact-mirror
build-ttl.sh: build-and-push-local-artifact-mirror-image

.PHONY: clean
clean:
	rm -rf bin build

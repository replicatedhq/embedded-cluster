SHELL := /bin/bash

include ../common.mk

OS ?= linux
ARCH ?= $(shell go env GOARCH)

USE_CHAINGUARD ?= 1

export PACKAGE_VERSION ?= $(VERSION)
export IMAGE_NAME ?= replicated/embedded-cluster-local-artifact-mirror
export IMAGE_TAG ?= $(shell echo "$(PACKAGE_VERSION)" | sed 's/+/-/')
export IMAGE ?= $(IMAGE_NAME):$(IMAGE_TAG)

.PHONY: build
build:
	mkdir -p bin
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build \
		-tags osusergo,netgo \
		-ldflags="-s -w -extldflags=-static" \
		-o bin/local-artifact-mirror-$(OS)-$(ARCH) ../cmd/local-artifact-mirror

export MELANGE_CONFIG := deploy/melange.tmpl.yaml

.PHONY: melange-build-local-artifact-mirror
melange-build-local-artifact-mirror:
	$(MAKE) melange-build MELANGE_SOURCE_DIR=../

export APKO_CONFIG := deploy/apko.tmpl.yaml

.PHONY: build-local-artifact-mirror-image
build-local-artifact-mirror-image:
	if [ "$(USE_CHAINGUARD)" == "1" ]; then \
		$(MAKE) build-local-artifact-mirror-image-chainguard; \
	else \
		$(MAKE) build-local-artifact-mirror-image-dockerfile; \
	fi

.PHONY: build-local-artifact-mirror-image-chainguard
build-local-artifact-mirror-image-chainguard: melange-build-local-artifact-mirror apko-build

.PHONY: build-local-artifact-mirror-image-dockerfile
build-local-artifact-mirror-image-dockerfile:
	docker build --platform="$(OS)/$(ARCH)" -t $(IMAGE) -f deploy/Dockerfile ../
	echo "$(IMAGE)" > build/image

.PHONY: build-and-push-local-artifact-mirror-image
build-and-push-local-artifact-mirror-image:
	if [ "$(USE_CHAINGUARD)" == "1" ]; then \
		$(MAKE) build-and-push-local-artifact-mirror-image-chainguard; \
	else \
		$(MAKE) build-and-push-local-artifact-mirror-image-dockerfile; \
	fi

.PHONY: build-and-push-local-artifact-mirror-image-chainguard
build-and-push-local-artifact-mirror-image-chainguard: melange-build-local-artifact-mirror apko-build-and-publish

.PHONY: build-and-push-local-artifact-mirror-image-dockerfile
build-and-push-local-artifact-mirror-image-dockerfile: build-local-artifact-mirror-image-dockerfile
	docker push $(IMAGE)

.PHONY: build-ttl.sh
build-ttl.sh:
	$(MAKE) build-and-push-local-artifact-mirror-image \
		IMAGE=ttl.sh/$(IMAGE)

.PHONY: clean
clean:
	rm -rf bin build
